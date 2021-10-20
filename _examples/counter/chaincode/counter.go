package chaincode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/datachainlab/fabric-batch-chaincode/batch"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type SmartContract struct {
	contractapi.Contract
	*batch.BatchContract
}

const (
	MsgIncr = "incr"
	MsgDecr = "decr"

	countKey = "count"
)

func NewSmartContract(batchcc *batch.BatchContract) *SmartContract {
	s := &SmartContract{
		BatchContract: batchcc,
	}
	batchcc.RegisterFn(MsgIncr, s.incrMsg)
	batchcc.RegisterFn(MsgDecr, s.decrMsg)
	return s
}

/// Smart Contract APIs ///

func (s *SmartContract) Init(ctx contractapi.TransactionContextInterface) error {
	log.Println("Init the contract!")
	return nil
}

func (s *SmartContract) Incr(ctx contractapi.TransactionContextInterface, count uint64) error {
	current, err := s.Count(ctx)
	if err != nil {
		return err
	}
	return setCount(ctx, current+count)
}

func (s *SmartContract) Decr(ctx contractapi.TransactionContextInterface, count uint64) error {
	current, err := s.Count(ctx)
	if err != nil {
		return err
	}
	if current < count {
		return fmt.Errorf("current value is less than %v: decr=%v", current, count)
	}
	return setCount(ctx, current-count)
}

func (s *SmartContract) Count(ctx contractapi.TransactionContextInterface) (uint64, error) {
	bz, err := ctx.GetStub().GetState(countKey)
	if err != nil {
		return 0, err
	}
	if len(bz) == 0 {
		return 0, nil
	}
	return binary.BigEndian.Uint64(bz), nil
}

/// Batch Contract APIs ///

func (s *SmartContract) BatchIncr(ctx contractapi.TransactionContextInterface, count uint64) error {
	tm, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return err
	}
	return s.BatchContract.SubmitMsg(ctx, batch.Msg{
		Fn: MsgIncr,
		Args: [][]byte{
			[]byte(fmt.Sprint(count)),
		},
	}, tm.GetSeconds())
}

func (s *SmartContract) BatchIncrWithTimestamp(ctx contractapi.TransactionContextInterface, count uint64, currentTime int64) error {
	return s.BatchContract.SubmitMsg(ctx, batch.Msg{
		Fn: MsgIncr,
		Args: [][]byte{
			[]byte(fmt.Sprint(count)),
		},
	}, currentTime)
}

func (s *SmartContract) incrMsg(ctx contractapi.TransactionContextInterface, args [][]byte) error {
	if len(args) != 1 {
		return errors.New("args length must be 1")
	}
	count, err := strconv.Atoi(string(args[0]))
	if err != nil {
		return err
	}
	return s.Incr(ctx, uint64(count))
}

func (s *SmartContract) BatchDecrWithTimestamp(ctx contractapi.TransactionContextInterface, count uint64, currentTime int64) error {
	return s.BatchContract.SubmitMsg(ctx, batch.Msg{
		Fn: MsgDecr,
		Args: [][]byte{
			[]byte(fmt.Sprint(count)),
		},
	}, currentTime)
}

func (s *SmartContract) decrMsg(ctx contractapi.TransactionContextInterface, args [][]byte) error {
	if len(args) != 1 {
		return errors.New("args length must be 1")
	}
	count, err := strconv.Atoi(string(args[0]))
	if err != nil {
		return err
	}
	return s.Decr(ctx, uint64(count))
}

/// internal functions ///

func (s *SmartContract) GetIgnoredFunctions() []string {
	return s.BatchContract.GetIgnoredFunctions()
}

func setCount(ctx contractapi.TransactionContextInterface, count uint64) error {
	var bz [8]byte
	binary.BigEndian.PutUint64(bz[:], count)
	return ctx.GetStub().PutState(countKey, bz[:])
}
