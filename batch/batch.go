package batch

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/hyperledger/fabric-chaincode-go/pkg/cid"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

const (
	lastCommittedTimeKey = "lct"
	msgKeyPrefix         = "msg/"
	commitKeyPrefix      = "commit/"
)

type BatchContract struct {
	Authenticator Authenticator
	FnRegistry    map[string]Fn

	// parameters for the batch algorithm
	TotalQueryLimit uint32
	TimeGapAllwance int64
	GetNodeTime     func() int64
}

var DefaultGetNodeTime = func() int64 {
	return time.Now().Unix()
}

/// Smart Contract API ///

// SubmitMsg inserts Msg for a key with `currentTime` prefix
// Theses Msgs will be committed by the commit function.
func (s *BatchContract) SubmitMsg(ctx contractapi.TransactionContextInterface, msg Msg, currentTime int64) error {
	// Validate a submitted Msg
	if err := msg.Validate(); err != nil {
		return err
	}
	if _, ok := s.FnRegistry[msg.Fn]; !ok {
		return fmt.Errorf("fnName '%v' not found in the registry", msg.Fn)
	}
	if err := s.authenticateMsg(ctx, msg); err != nil {
		return err
	}

	// Validate a gap between `currentTime` and node time
	if err := validateCurrentTimestamp(currentTime, s.GetNodeTime(), s.TimeGapAllwance); err != nil {
		return err
	}

	// Verify that no commits exist after `currentTime`
	if err := verifyNotExpired(ctx, currentTime); err != nil {
		return err
	}

	// Submit the msg
	if err := addMsg(ctx, currentTime, msg); err != nil {
		return err
	}
	return nil
}

// Commit executes Msg submitted between the last commit time and `commitTime`.
func (s *BatchContract) Commit(ctx contractapi.TransactionContextInterface, commitTime int64) error {
	if err := validateCommitTime(commitTime, s.GetNodeTime(), s.TimeGapAllwance); err != nil {
		return err
	}
	lct, err := getLastCommittedTime(ctx)
	if err != nil {
		return err
	}
	if commitTime <= lct {
		return fmt.Errorf("commit %v has already committed: lct=%v", commitTime, lct)
	}
	iter, err := ctx.GetStub().GetStateByRange(makeMsgRangeKey(lct, commitTime))
	if err != nil {
		return err
	}
	defer iter.Close()
	var count uint32 = 0
	state := NewBatchState(ctx.GetStub())
	for iter.HasNext() {
		res, err := iter.Next()
		if err != nil {
			return err
		}
		var msg Msg
		if err := json.Unmarshal(res.Value, &msg); err != nil {
			return err
		}
		if err := s.executeMsg(ctx, state, msg); err != nil {
			return err
		}
		// cleanup
		if err := ctx.GetStub().DelState(res.Key); err != nil {
			return err
		}
		count++
		if s.TotalQueryLimit <= count {
			return fmt.Errorf("the number of results must be less than `TotalQueryLimit` %v: count=%v", s.TotalQueryLimit, count)
		}
	}
	if err := state.Commit(); err != nil {
		return err
	}
	if err := setCommit(ctx, commitTime); err != nil {
		return err
	}
	return setLastCommittedTime(ctx, commitTime)
}

/// API for subclass ///

func (s *BatchContract) RegisterFn(fnName string, fn Fn) error {
	if _, ok := s.FnRegistry[fnName]; ok {
		return fmt.Errorf("fnName '%v' already exists in the registry", fnName)
	}
	s.FnRegistry[fnName] = fn
	return nil
}

func (s *BatchContract) GetIgnoredFunctions() []string {
	return []string{"RegisterFn", "SubmitMsg"}
}

/// Msg handlers ///

func (s *BatchContract) authenticateMsg(ctx contractapi.TransactionContextInterface, msg Msg) error {
	return s.Authenticator(ctx, msg)
}

func (s *BatchContract) executeMsg(ctx contractapi.TransactionContextInterface, state *BatchState, msg Msg) error {
	fn, ok := s.FnRegistry[msg.Fn]
	if !ok {
		return fmt.Errorf("fnName '%v' not found in the registry", msg.Fn)
	}
	tmp := ctx.(*contractapi.TransactionContext)
	tctx := *tmp
	tctx.SetStub(state)
	if err := fn(&tctx, msg.Args); err != nil {
		if err := state.RevertMsg(); err != nil {
			return err
		}
	} else {
		if err := state.CommitMsg(); err != nil {
			return err
		}
	}
	return nil
}

/// State accessors ///

func getLastCommittedTime(ctx contractapi.TransactionContextInterface) (int64, error) {
	bz, err := ctx.GetStub().GetState(lastCommittedTimeKey)
	if err != nil {
		return 0, err
	} else if bz == nil {
		return 0, nil
	}
	return int64(binary.BigEndian.Uint64(bz)), nil
}

func setLastCommittedTime(ctx contractapi.TransactionContextInterface, lct int64) error {
	lctBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(lctBytes[:], uint64(lct))
	return ctx.GetStub().PutState(lastCommittedTimeKey, lctBytes)
}

func addMsg(ctx contractapi.TransactionContextInterface, currentTime int64, msg Msg) error {
	bz, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	h := sha256.Sum256(bz)
	return ctx.GetStub().PutState(makeMsgKey(currentTime, h[:]), bz)
}

func setCommit(ctx contractapi.TransactionContextInterface, currentTime int64) error {
	return ctx.GetStub().PutState(makeCommitKey(currentTime), []byte{1})
}

/// Validation functions ///

// verifyNotExpired verifies that the `currentTime` can be included in the next commit.
func verifyNotExpired(ctx contractapi.TransactionContextInterface, currentTime int64) error {
	// By calling GetStateByRange with `currentTime` as start-key, this transaction fails
	// if there is a commit that cover a newer time in concurrent during validation phase.
	iter, err := ctx.GetStub().GetStateByRange(makeCommitKey(currentTime), makeCommitEndKey())
	if err != nil {
		return err
	}
	defer iter.Close()
	if iter.HasNext() {
		return fmt.Errorf("the current time '%v' is already expired", currentTime)
	}
	return nil
}

func validateCurrentTimestamp(currentTime int64, nodeTime int64, timeGapAllowance int64) error {
	if nodeTime-timeGapAllowance <= currentTime && currentTime <= nodeTime+timeGapAllowance {
		return nil
	} else {
		return fmt.Errorf("time %v is outside the acceptable range: timeGapAllowance=%v nodeTime=%v", currentTime, timeGapAllowance, nodeTime)
	}
}

// validateCommitTime checks following:
// check if that commitTime is less than nodeTime
func validateCommitTime(commitTime int64, nodeTime int64, timeGapAllowance int64) error {
	// TODO introduce a minimum commit interval parameter?
	if commitTime < nodeTime-timeGapAllowance {
		return nil
	}
	return fmt.Errorf("commit is not ready")
}

/// Utility functions ///

func makeMsgPrefixKey(timestamp int64) string {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz[:], uint64(timestamp))
	return fmt.Sprintf("%v/%v/", msgKeyPrefix, hex.EncodeToString(bz))
}

func makeMsgKey(timestamp int64, msgHash []byte) string {
	return makeMsgPrefixKey(timestamp) + hex.EncodeToString(msgHash)
}

func makeMsgRangeKey(start, end int64) (string, string) {
	if start > end {
		panic("unexpected inputs")
	}
	return makeMsgPrefixKey(start), makeMsgPrefixKey(end)
}

func makeCommitKey(timestamp int64) string {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz[:], uint64(timestamp))
	return fmt.Sprintf("%v/%v", commitKeyPrefix, hex.EncodeToString(bz))
}

func makeCommitEndKey() string {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz[:], math.MaxUint64)
	return fmt.Sprintf("%v/%v", commitKeyPrefix, hex.EncodeToString(bz))
}

func newTransactionContext(stub shim.ChaincodeStubInterface) (contractapi.TransactionContextInterface, error) {
	var ctx contractapi.TransactionContext
	ctx.SetStub(stub)
	ci, err := cid.New(stub)
	if err != nil {
		return nil, err
	}
	ctx.SetClientIdentity(ci)
	return &ctx, nil
}

/// Msg definitions ///

type Fn func(contractapi.TransactionContextInterface, [][]byte) error

type Authenticator func(contractapi.TransactionContextInterface, Msg) error

func DefaultAuthenticator(ctx contractapi.TransactionContextInterface, msg Msg) error {
	return nil
}

type Msg struct {
	Fn   string
	Args [][]byte
}

func (msg Msg) Validate() error {
	if msg.Fn == "" {
		return errors.New("Fn must be non-empty")
	}
	return nil
}
