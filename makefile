IMG ?= cloudoperators/khaos


# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet
	go run ./pkg/main.go


# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy:
	kubectl apply -f deploy 

# Run go fmt against code
fmt:
	go fmt ./pkg/... 

# Run go vet against code
vet:
	go vet ./pkg/... 

# Build the docker image
docker-build: check-tag
	docker build --no-cache . -t ${IMG}:${TAG}
	
# Push the docker image
docker-push: check-tag
	echo "${DOCKER_PASSWORD}" | docker login -u "${DOCKER_USER}" --password-stdin
	docker push ${IMG}:${TAG}

check-tag:
ifndef TAG
	$(error TAG is undefined! Please set TAG to the latest release tag, using the format x.y.z e.g. export TAG=0.1.1 ) 
endif