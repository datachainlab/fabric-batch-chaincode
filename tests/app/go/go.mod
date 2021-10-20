module github.com/fabricgoapp

go 1.15

require github.com/hyperledger/fabric-sdk-go v1.0.0-beta3

replace (
	github.com/fabricgoapp => ../
	github.com/hyperledger/fabric-sdk-go => github.com/datachainlab/fabric-sdk-go v1.0.0-beta2.0.20211020062434-ea4fb5be7a93
)
