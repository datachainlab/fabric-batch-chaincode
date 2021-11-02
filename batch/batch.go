package batch

import (
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

// BatchContract implements on-chain batch logic without trusted third party
type BatchContract struct {
	Authenticator Authenticator
	FnRegistry    map[string]Fn

	// parameters for the batch algorithm
	TotalQueryLimit        uint32 // the value should be match `totalQueryLimit` in core.yaml
	MsgTimeGapAllowance    int64  // the value indicates current time window when Msg is submitted
	CommitTimeGapAllowance int64  // the value indicates current time window when Commit is executed

	// GetPeerTime returns current time on peer
	GetPeerTime func(stub shim.ChaincodeStubInterface) int64
}

// DefaultGetPeerTime uses local clock in peer
func DefaultGetPeerTime(stub shim.ChaincodeStubInterface) int64 {
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
	if err := validateCurrentTimestamp(currentTime, s.GetPeerTime(ctx.GetStub()), s.MsgTimeGapAllowance); err != nil {
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
func (s *BatchContract) Commit(ctx contractapi.TransactionContextInterface, commitTime int64) (int64, error) {
	if err := validateCommitTime(commitTime, s.GetPeerTime(ctx.GetStub()), s.CommitTimeGapAllowance); err != nil {
		return 0, err
	}
	lct, err := getLastCommittedTime(ctx)
	if err != nil {
		return 0, err
	}
	if commitTime <= lct {
		return 0, fmt.Errorf("commit %v has already committed: lct=%v", commitTime, lct)
	}
	iter, err := ctx.GetStub().GetStateByRange(makeMsgRangeKey(lct, commitTime))
	if err != nil {
		return 0, err
	}
	defer iter.Close()
	var count int64 = 0
	state := NewBatchState(ctx.GetStub())
	for iter.HasNext() {
		res, err := iter.Next()
		if err != nil {
			return 0, err
		}
		var msg Msg
		if err := json.Unmarshal(res.Value, &msg); err != nil {
			return 0, err
		}
		if err := s.executeMsg(ctx, state, msg); err != nil {
			return 0, err
		}
		// cleanup
		if err := ctx.GetStub().DelState(res.Key); err != nil {
			return 0, err
		}
		count++
		if int64(s.TotalQueryLimit) <= count {
			return 0, fmt.Errorf("the number of results must be less than `TotalQueryLimit` %v: count=%v", s.TotalQueryLimit, count)
		}
	}
	if err := state.Apply(); err != nil {
		return 0, err
	}
	if err := setCommit(ctx, commitTime); err != nil {
		return 0, err
	}
	if err := setLastCommittedTime(ctx, commitTime); err != nil {
		return 0, err
	}
	return count, nil
}

/// API for subclass ///

// RegisterFn registers a given function
func (s *BatchContract) RegisterFn(fnName string, fn Fn) error {
	if _, ok := s.FnRegistry[fnName]; ok {
		return fmt.Errorf("fnName '%v' already exists in the registry", fnName)
	}
	s.FnRegistry[fnName] = fn
	return nil
}

// GetIgnoredFunctions implements contractapi.IgnoreContractInterface
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
	return ctx.GetStub().PutState(makeMsgKey(currentTime, ctx.GetStub().GetTxID()), bz)
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
	return fmt.Errorf("commit is not ready: commitTime=%v nodeTime=%v timeGapAllowance=%v", commitTime, nodeTime, timeGapAllowance)
}

/// Utility functions ///

func makeMsgPrefixKey(timestamp int64) string {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz[:], uint64(timestamp))
	return fmt.Sprintf("%v/%v/", msgKeyPrefix, hex.EncodeToString(bz))
}

func makeMsgKey(timestamp int64, txID string) string {
	return makeMsgPrefixKey(timestamp) + txID
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

// Fn defines a function to process Msg
type Fn func(ctx contractapi.TransactionContextInterface, args [][]byte) error

// Authenticator defines an authenticator that authenticates a given msg with a transaction context
type Authenticator func(ctx contractapi.TransactionContextInterface, msg Msg) error

// DefaultAuthenticator ...
func DefaultAuthenticator(ctx contractapi.TransactionContextInterface, msg Msg) error {
	return nil
}

// Msg is a chaincode invocation
type Msg struct {
	Fn   string
	Args [][]byte
}

// Validate validates the msg
func (msg Msg) Validate() error {
	if msg.Fn == "" {
		return errors.New("Fn must be non-empty")
	}
	return nil
}
