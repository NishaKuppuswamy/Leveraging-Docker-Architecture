COEN 241: CLOUD COMPUTING
TEAM : 8
PROJECT TITLE: Leveraging Docker architecture for consistent development and Deployment
ENVIRONMENT: AWS LINUX AMI
STEPS:
Install Docker steps:
	sudo yum update -y
	sudo yum install -y docker
	sudo service docker start
	sudo usermod -a -G docker ec2-user
	sudo yum install -y git
	sudo yum -y groupinstall "Development Tools"
	refer: https://gist.github.com/brianz/8458fc666f5156fdbbc2
Create Private Docker Registery:
	sudo yum install docker-registry
	firewall-cmd --permanent --add-port=5000/tcp
	firewall-cmd --reload
	systemctl start docker-registry
	systemctl enable docker-registry
	systemctl status docker-registry
	 ADD_REGISTRY='--add-registry localhost:5000'
	 INSECURE_REGISTRY='--insecure-registry localhost:5000'
 	systemctl restart docker
	refer: http://suraj.pro/post/blog11/
Install Go:
	sudo yum install golang
	sudo vi ~/.profile
	ADD following line to profile :
	export PATH=$PATH:/usr/local/go/bin
	$ mkdir -p $HOME/go_projects/{src,pkg,bin}
	sudo vi ~/.profile
ADD following line to profile :
	export GOPATH="$HOME/go_projects"
	export GOBIN="$GOPATH/bin"
	source ~/.profile
	go version  -> to check go installation
	refer: https://www.ostechnix.com/install-go-language-linux/
FID related installation steps:
	sudo yum install zlib1g-dev make g++ ctorrent
	git clone https://github.com/gebi/libowfat
	cd libowfat
	make
	git clone git://erdgeist.org/opentracker
	cd opentracker
	make
Save FID project folder inside following go path:
	cd go_projects/src/FID
Pull a docker image to local client machine:
	docker pull tomcat:latest
Open new terminal to run Open Tracker:
	cd Opentracker
	sudo ./opentracker -i 0.0.0.0 -p 8940
	Open new terminal to run Registry:
	cd go_projects/src/FID/registry/
	sudo ./registry 
Open new terminal for client to test docker push through FID:
	cd go_projects/src/FID/client/
	sudo ./client push tomcat:latest
Open new terminal for client to test docker pull for FID:
	cd go_projects/src/FID/client/
	sudo ./client pull tomcat:latest


	

