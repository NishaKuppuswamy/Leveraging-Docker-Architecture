package main

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kingpin"
	"github.com/codegangsta/martini"
	"github.com/jackpal/Taipei-Torrent/torrent"
        //"github.com/fsouza/go-dockerclient"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

var (
	tracker  = kingpin.Flag("tracker", "Set host and port of bittorrent tracker. Example: -host 10.240.101.85:8940 Note: This cannot be set to localhost, since this is the tracker in which all the torrents will be created with. They have to be some accessible ip address from outside").Short('t').Default("0.0.0.0:8940").String()
	port     = kingpin.Flag("port", "Set port of docket registry.").Short('p').Default("8004").String()
	location = kingpin.Flag("location", "Set location to save torrents and docker images.").Short('l').Default("/var/local/docket").String()
)

// The one and only martini instance.
var store *Store
var torrentstore=make(map[string]string)
 
var m *martini.Martini

func init() {
	m = martini.New()
	// Setup routes
	r := martini.NewRouter()
	r.Post(`/images`, postImage)
	r.Get(`/torrents`, getTorrent)
	r.Get(`/images/all`, getImagesList)
	r.Get(`/images`, getImages)
	// Add the router action
	m.Action(r.Handle)
}

func postImage(w http.ResponseWriter, r *http.Request) (int, string) {
	w.Header().Set("Content-Type", "application/json")

	loc := *location
	//fmt.Println("location, ", loc)

	// the FormFile function takes in the POST input id file
	file, header, err := r.FormFile("file")

	if err != nil {
		fmt.Println(err)
		return 500, "bad"
	}

	defer file.Close()

	//Get metadata
	image := r.Header.Get("image")
	id := r.Header.Get("id")
	created := r.Header.Get("created")
	fileName := header.Filename
        
	//fmt.Println("Got image: ", image, " id = ", id, " created = ", created, " filename = ", fileName)

	s := []string{loc, "/", fileName}
	t := []string{loc, "/", fileName, ".torrent"}
	filePath := strings.Join(s, "")
	torrentFile := fileName + ".torrent"
	torrentPath := strings.Join(t, "")

	out, err := os.Create(filePath)
	if err != nil {
		fmt.Println(err)
		return 500, "bad"
	}

	defer out.Close()

	// write the content from POST to the file
	_, err = io.Copy(out, file)
	if err != nil {
		fmt.Println(err)
		return 500, "bad"
	}

	fmt.Printf("Tar Image of %s uploaded successfully", image)

	fmt.Println("Extracting the tar file")
        imageTar := loc+"/"+fileName
        cmdStr1 := "sudo mkdir "+loc+"/test1 && sudo tar -xf " + imageTar + " -C "+loc+"/test1"
        _ , err1 := exec.Command("sh", "-c", cmdStr1).Output()
        if err1 != nil {
          fmt.Printf("ERROR: %s", err1)
        }
	tempFolder := "test1/"
	srcLayer := loc + "/" + tempFolder
	files, err := ioutil.ReadDir(srcLayer)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Println("Storing the Manifest, Repositories in DataStore")
	metadata, errM := ioutil.ReadFile(srcLayer+"manifest.json")
	if errM != nil {
		fmt.Print(metadata)
	}
	tempN := strings.Split(fileName, "_")[0]
	jsonFileName := strings.Split(tempN, ":")[1]
	jsonFile, errJ := ioutil.ReadFile(srcLayer+jsonFileName+".json")
	if errJ != nil {
		fmt.Print(jsonFile)
	}

	repository, errR := ioutil.ReadFile(srcLayer+"repositories")
	if errR != nil {
		fmt.Print(repository)
	}
	

	btHost := *tracker
	layers := ""
	layerMap := ""
	os.Chdir(loc)
	for _, file := range files {
		//fmt.Println("\n",file.Name(),"\n")
		if file.Name() == "repositories" || strings.HasSuffix(file.Name(), ".json"){
			continue
		}
		fmt.Printf("\nFind the ShaSum256 for each layer: %s\n", file.Name())
		cmdStrS := "sudo sha256sum "+ srcLayer+file.Name() + "/layer.tar"
		cmd := exec.Command("sh", "-c", cmdStrS)
		out, err := cmd.CombinedOutput()
		temp := strings.Split(string(out), " ")[0]
		if err != nil{
			fmt.Print(err)
		}
		if layers == "" {
			layers = file.Name()
			layerMap = file.Name()+":"+temp
		}else{
			layers = layers + "," +file.Name()
			layerMap = layerMap + "," + file.Name()+":"+temp
		}
		
		os.Chdir(srcLayer)
		cmdStr1 := "sudo tar -cf  "+ loc+"/"+file.Name()+".tar " + file.Name()
		_ , err1 := exec.Command("sh", "-c", cmdStr1).Output()
		if err1 != nil {
		  fmt.Printf("ERROR: %s", err1)
		}
		
		os.Chdir(loc)
		cmdStr2 := "ctorrent -t "+ file.Name()+".tar -s " + file.Name() + ".tar.torrent -u http://"+btHost+"/announce" 
		_ , errT := exec.Command("sh", "-c", cmdStr2).Output()
		if errT != nil {
		  fmt.Printf("ERROR: %s", errT)
		}
		torrentstore[file.Name()] = image
		fmt.Print("\nCreating torrent for each layer and seeding\n")
		importCmd := fmt.Sprintf("sudo ctorrent -d -e 9999 %s", file.Name()+".tar.torrent")
		//fmt.Print("\n\n\n\n\nSeeding the torrent--  ",importCmd )
		_, err2 := exec.Command("sh", "-c", importCmd).Output()
		if err2 != nil {
			fmt.Printf("Failed to seed torrent..")
			fmt.Println(err2)
			return 500, "bad"
		}
	}
	os.RemoveAll(srcLayer)

	//JSON string of metadata
	imageMeta := map[string]string{
		"image":    image,
		"id":       id,
		"created":  created,
		"fileName": fileName,
		"layers" : layers,
		"layerMap": layerMap,
		"jsonFile" : string(jsonFile),
		"metadata": string(metadata),
		"repository":string(repository),
	}
	imageMetaJson, _ := json.Marshal(imageMeta)

	//Write to datastore
	err = writeToStore(store, "docket", image, string(imageMetaJson))
	if err != nil {
		fmt.Println("Error writing result: ", err)
	}

	err = createTorrentFile(torrentPath, filePath, btHost)
	if err != nil {
		return 500, "torrent creation failed"
	}
	
	//Seed the torrent
	fmt.Println("Seeding torrent for main image in the background...")
	os.Chdir(loc)

	importCmd := fmt.Sprintf("sudo ctorrent -d -e 9999 %s", torrentFile)
	_, err2 := exec.Command("sh", "-c", importCmd).Output()
	if err2 != nil {
		fmt.Printf("Failed to seed torrent..")
		fmt.Println(err2)
		return 500, "bad"
	}

	return http.StatusOK, "{\"status\":\"OK\"}"
}

func getTorrent(w http.ResponseWriter, r *http.Request) int {
	query := r.URL.Query()
	queryJson := query.Get("q")

	var queryObj map[string]interface{}
	if err := json.Unmarshal([]byte(queryJson), &queryObj); err != nil {
		return 500
	}

	imageInterface := queryObj["image"]
	image := imageInterface.(string)
	torrentFile := ""
	if _, ok := torrentstore[image]; ok {
		//fmt.Print("---",val)
		torrentFile = image + ".tar.torrent"
	}else{

		//Query db and find if image exists. If not throw error (done)
		jsonVal, err := getFromStore(store, "docket", image)
		if err != nil {
			fmt.Println("Error reading from file : %v\n", err)
			return 500
		}

		if jsonVal == "" {
			fmt.Println("Invalid image requested")
			return 500
		}


		//Unmarshall
		var imageObj map[string]interface{}
		if err := json.Unmarshal([]byte(jsonVal), &imageObj); err != nil {
			return 500
		}
		//find location to torrent
		torrentFileInterface := imageObj["fileName"]
		torrentFile = torrentFileInterface.(string) + ".torrent"
		
	}
	

	torrentPath := *location + "/" + torrentFile
	//Check if file exists
	if _, err := os.Stat(torrentPath); os.IsNotExist(err) {
		fmt.Println("no such file or directory: %s", torrentPath)
		return 500
	}

	//set filepath to that
	file, err := ioutil.ReadFile(torrentPath)
	if err != nil {
		return 500
	}

	w.Header().Set("Content-Type", "application/x-bittorrent")
	if file != nil {
		w.Write(file)
		return http.StatusOK
	}

	return 500
}

func getImages(w http.ResponseWriter, r *http.Request) (int, string) {
	query := r.URL.Query()
	queryJson := query.Get("q")

	var queryObj map[string]interface{}
	if err := json.Unmarshal([]byte(queryJson), &queryObj); err != nil {
		return 500, ""
	}

	imageInterface := queryObj["image"]
	image := imageInterface.(string)

	fmt.Println("image = ", image)

	//Query db and find if image exists. If not throw error (done)
	jsonVal, err := getFromStore(store, "docket", image)
	if err != nil {
		fmt.Println("Error reading from file : %v\n", err)
		return 500, ""
	}

	if jsonVal == "" {
		fmt.Println("Invalid image requested")
		return 500, ""
	}

	w.Header().Set("Content-Type", "application/json")
	return http.StatusOK, jsonVal
}

func getImagesList(w http.ResponseWriter, r *http.Request) (int, string) {
	//Query db and find if image exists. If not throw error (done)
	keys, err := iterateStore(store, "docket")
	if err != nil {
		fmt.Println("Error reading from file : %v\n", err)
		return 500, ""
	}

	if keys == "" {
		fmt.Println("Invalid image requested")
		return 500, ""
	}

	w.Header().Set("Content-Type", "text/plain")
	return http.StatusOK, keys
}

func createTorrentFile(torrentFileName, root, announcePath string) (err error) {
	var metaInfo *torrent.MetaInfo
	metaInfo, err = torrent.CreateMetaInfoFromFileSystem(nil, root, "0.0.0.0:8940", 0, false)
	if err != nil {
		return
	}
	btHost := *tracker
	metaInfo.Announce = "http://" + btHost + "/announce"
	metaInfo.CreatedBy = "docket-registry"
	var torrentFile *os.File
	torrentFile, err = os.Create(torrentFileName)
	if err != nil {
		return
	}
	defer torrentFile.Close()
	err = metaInfo.Bencode(torrentFile)
	if err != nil {
		return
	}
	return
}

func main() {
	
	kingpin.Parse()

	loc := *location
	if _, err := os.Stat(loc); os.IsNotExist(err) {
		os.Mkdir(loc, 0644)
	}

	var storeErr error

	store, storeErr = openStore()
	if storeErr != nil {
		log.Fatal("Failed to open data store: %v", storeErr)
	}
	deferCloseStore(store)

	pString := ":" + *port

	if err := http.ListenAndServe(pString, m); err != nil {
		log.Fatal(err)
	}
}

