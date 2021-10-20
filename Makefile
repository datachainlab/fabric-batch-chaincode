FABRIC_VERSION ?= 2.0.0

minifab:
	curl -o minifab -L https://tinyurl.com/twrt8zv && chmod +x minifab

.PHONY: minifab-init
minifab-init: minifab
	./minifab imageget -i $(FABRIC_VERSION)
	rm -rf ./vars/chaincode/*
	$(MAKE) testchaincode

.PHONY: minifab-up
minifab-up:
	./minifab up -n counter -l go -v 1.0 -p '"SmartContract:init"'

.PHONY: minifab-cleanup
minifab-cleanup:
	./minifab cleanup

.PHONY: testapp
testapp:
	rm -rf ./vars/app/go
	cp -a ./tests/app/go ./vars/app/go

.PHONY: testchaincode
testchaincode:
	rm -rf ./vars/chaincode/counter/go && mkdir -p ./vars/chaincode/counter/go
	cp -a ./main.go ./batch ./_examples ./go.* ./vars/chaincode/counter/go
