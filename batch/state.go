package batch

import (
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	pb "github.com/hyperledger/fabric-protos-go/peer"
)

// BatchState manages a state in a batch execution
type BatchState struct {
	stub shim.ChaincodeStubInterface

	committed   map[string][]byte // prefix: 0x01: cache, 0x02: updated, 0x03: deleted
	provisional map[string][]byte
}

const (
	prefixCache   byte = 0x01
	prefixUpdated byte = 0x02
	prefixDeleted byte = 0x03
)

var _ shim.ChaincodeStubInterface = (*BatchState)(nil)

// NewBatchState returns a new BatchState
func NewBatchState(stub shim.ChaincodeStubInterface) *BatchState {
	return &BatchState{
		stub:        stub,
		committed:   make(map[string][]byte),
		provisional: make(map[string][]byte),
	}
}

// CommitMsg commits the provisional state
func (s *BatchState) CommitMsg() error {
	for k, v := range s.provisional {
		if v == nil {
			s.committed[k] = []byte{prefixDeleted}
		} else {
			s.committed[k] = append([]byte{prefixUpdated}, v...)
		}
	}
	s.provisional = make(map[string][]byte)
	return nil
}

// RevertMsg discards the provisional state
func (s *BatchState) RevertMsg() error {
	s.provisional = make(map[string][]byte)
	return nil
}

// Apply applies the committed state to the state DB
func (s *BatchState) Apply() error {
	for k, v := range s.committed {
		switch v[0] {
		case prefixCache:
		case prefixUpdated:
			if err := s.stub.PutState(k, v[1:]); err != nil {
				return err
			}
		case prefixDeleted:
			if err := s.stub.DelState(k); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetArgs returns the arguments intended for the chaincode Init and Invoke
// as an array of byte arrays.
func (s *BatchState) GetArgs() [][]byte {
	panic("not implemented") // TODO: Implement
}

// GetStringArgs returns the arguments intended for the chaincode Init and
// Invoke as a string array. Only use GetStringArgs if the client passes
// arguments intended to be used as strings.
func (s *BatchState) GetStringArgs() []string {
	panic("not implemented") // TODO: Implement
}

// GetFunctionAndParameters returns the first argument as the function
// name and the rest of the arguments as parameters in a string array.
// Only use GetFunctionAndParameters if the client passes arguments intended
// to be used as strings.
func (s *BatchState) GetFunctionAndParameters() (string, []string) {
	panic("not implemented") // TODO: Implement
}

// GetArgsSlice returns the arguments intended for the chaincode Init and
// Invoke as a byte array
func (s *BatchState) GetArgsSlice() ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

// GetTxID returns the tx_id of the transaction proposal, which is unique per
// transaction and per client. See
// https://godoc.org/github.com/hyperledger/fabric-protos-go/common#ChannelHeader
// for further details.
func (s *BatchState) GetTxID() string {
	panic("not implemented") // TODO: Implement
}

// GetChannelID returns the channel the proposal is sent to for chaincode to process.
// This would be the channel_id of the transaction proposal (see
// https://godoc.org/github.com/hyperledger/fabric-protos-go/common#ChannelHeader )
// except where the chaincode is calling another on a different channel.
func (s *BatchState) GetChannelID() string {
	panic("not implemented") // TODO: Implement
}

// InvokeChaincode locally calls the specified chaincode `Invoke` using the
// same transaction context; that is, chaincode calling chaincode doesn't
// create a new transaction message.
// If the called chaincode is on the same channel, it simply adds the called
// chaincode read set and write set to the calling transaction.
// If the called chaincode is on a different channel,
// only the Response is returned to the calling chaincode; any PutState calls
// from the called chaincode will not have any effect on the ledger; that is,
// the called chaincode on a different channel will not have its read set
// and write set applied to the transaction. Only the calling chaincode's
// read set and write set will be applied to the transaction. Effectively
// the called chaincode on a different channel is a `Query`, which does not
// participate in state validation checks in subsequent commit phase.
// If `channel` is empty, the caller's channel is assumed.
func (s *BatchState) InvokeChaincode(chaincodeName string, args [][]byte, channel string) pb.Response {
	panic("not implemented") // TODO: Implement
}

// GetState returns the value of the specified `key` from the
// ledger. Note that GetState doesn't read data from the writeset, which
// has not been committed to the ledger. In other words, GetState doesn't
// consider data modified by PutState that has not been committed.
// If the key does not exist in the state database, (nil, nil) is returned.
func (s *BatchState) GetState(key string) ([]byte, error) {
	var err error
	v, ok := s.committed[key]
	if !ok {
		v, err = s.stub.GetState(key)
		if err != nil {
			return nil, err
		}
		s.committed[key] = append([]byte{prefixCache}, v...)
		return v, nil
	}
	return v[1:], nil
}

// PutState puts the specified `key` and `value` into the transaction's
// writeset as a data-write proposal. PutState doesn't effect the ledger
// until the transaction is validated and successfully committed.
// Simple keys must not be an empty string and must not start with a
// null character (0x00) in order to avoid range query collisions with
// composite keys, which internally get prefixed with 0x00 as composite
// key namespace. In addition, if using CouchDB, keys can only contain
// valid UTF-8 strings and cannot begin with an underscore ("_").
func (s *BatchState) PutState(key string, value []byte) error {
	s.provisional[key] = value
	return nil
}

// DelState records the specified `key` to be deleted in the writeset of
// the transaction proposal. The `key` and its value will be deleted from
// the ledger when the transaction is validated and successfully committed.
func (s *BatchState) DelState(key string) error {
	s.provisional[key] = nil
	return nil
}

// SetStateValidationParameter sets the key-level endorsement policy for `key`.
func (s *BatchState) SetStateValidationParameter(key string, ep []byte) error {
	panic("not implemented") // TODO: Implement
}

// GetStateValidationParameter retrieves the key-level endorsement policy
// for `key`. Note that this will introduce a read dependency on `key` in
// the transaction's readset.
func (s *BatchState) GetStateValidationParameter(key string) ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

// GetStateByRange returns a range iterator over a set of keys in the
// ledger. The iterator can be used to iterate over all keys
// between the startKey (inclusive) and endKey (exclusive).
// However, if the number of keys between startKey and endKey is greater than the
// totalQueryLimit (defined in core.yaml), this iterator cannot be used
// to fetch all keys (results will be capped by the totalQueryLimit).
// The keys are returned by the iterator in lexical order. Note
// that startKey and endKey can be empty string, which implies unbounded range
// query on start or end.
// Call Close() on the returned StateQueryIteratorInterface object when done.
// The query is re-executed during validation phase to ensure result set
// has not changed since transaction endorsement (phantom reads detected).
func (s *BatchState) GetStateByRange(startKey string, endKey string) (shim.StateQueryIteratorInterface, error) {
	panic("not implemented") // TODO: Implement
}

// GetStateByRangeWithPagination returns a range iterator over a set of keys in the
// ledger. The iterator can be used to fetch keys between the startKey (inclusive)
// and endKey (exclusive).
// When an empty string is passed as a value to the bookmark argument, the returned
// iterator can be used to fetch the first `pageSize` keys between the startKey
// (inclusive) and endKey (exclusive).
// When the bookmark is a non-emptry string, the iterator can be used to fetch
// the first `pageSize` keys between the bookmark (inclusive) and endKey (exclusive).
// Note that only the bookmark present in a prior page of query results (ResponseMetadata)
// can be used as a value to the bookmark argument. Otherwise, an empty string must
// be passed as bookmark.
// The keys are returned by the iterator in lexical order. Note
// that startKey and endKey can be empty string, which implies unbounded range
// query on start or end.
// Call Close() on the returned StateQueryIteratorInterface object when done.
// This call is only supported in a read only transaction.
func (s *BatchState) GetStateByRangeWithPagination(startKey string, endKey string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *pb.QueryResponseMetadata, error) {
	panic("not implemented") // TODO: Implement
}

// GetStateByPartialCompositeKey queries the state in the ledger based on
// a given partial composite key. This function returns an iterator
// which can be used to iterate over all composite keys whose prefix matches
// the given partial composite key. However, if the number of matching composite
// keys is greater than the totalQueryLimit (defined in core.yaml), this iterator
// cannot be used to fetch all matching keys (results will be limited by the totalQueryLimit).
// The `objectType` and attributes are expected to have only valid utf8 strings and
// should not contain U+0000 (nil byte) and U+10FFFF (biggest and unallocated code point).
// See related functions SplitCompositeKey and CreateCompositeKey.
// Call Close() on the returned StateQueryIteratorInterface object when done.
// The query is re-executed during validation phase to ensure result set
// has not changed since transaction endorsement (phantom reads detected).
func (s *BatchState) GetStateByPartialCompositeKey(objectType string, keys []string) (shim.StateQueryIteratorInterface, error) {
	panic("not implemented") // TODO: Implement
}

// GetStateByPartialCompositeKeyWithPagination queries the state in the ledger based on
// a given partial composite key. This function returns an iterator
// which can be used to iterate over the composite keys whose
// prefix matches the given partial composite key.
// When an empty string is passed as a value to the bookmark argument, the returned
// iterator can be used to fetch the first `pageSize` composite keys whose prefix
// matches the given partial composite key.
// When the bookmark is a non-emptry string, the iterator can be used to fetch
// the first `pageSize` keys between the bookmark (inclusive) and the last matching
// composite key.
// Note that only the bookmark present in a prior page of query result (ResponseMetadata)
// can be used as a value to the bookmark argument. Otherwise, an empty string must
// be passed as bookmark.
// The `objectType` and attributes are expected to have only valid utf8 strings
// and should not contain U+0000 (nil byte) and U+10FFFF (biggest and unallocated
// code point). See related functions SplitCompositeKey and CreateCompositeKey.
// Call Close() on the returned StateQueryIteratorInterface object when done.
// This call is only supported in a read only transaction.
func (s *BatchState) GetStateByPartialCompositeKeyWithPagination(objectType string, keys []string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *pb.QueryResponseMetadata, error) {
	panic("not implemented") // TODO: Implement
}

// CreateCompositeKey combines the given `attributes` to form a composite
// key. The objectType and attributes are expected to have only valid utf8
// strings and should not contain U+0000 (nil byte) and U+10FFFF
// (biggest and unallocated code point).
// The resulting composite key can be used as the key in PutState().
func (s *BatchState) CreateCompositeKey(objectType string, attributes []string) (string, error) {
	panic("not implemented") // TODO: Implement
}

// SplitCompositeKey splits the specified key into attributes on which the
// composite key was formed. Composite keys found during range queries
// or partial composite key queries can therefore be split into their
// composite parts.
func (s *BatchState) SplitCompositeKey(compositeKey string) (string, []string, error) {
	panic("not implemented") // TODO: Implement
}

// GetQueryResult performs a "rich" query against a state database. It is
// only supported for state databases that support rich query,
// e.g.CouchDB. The query string is in the native syntax
// of the underlying state database. An iterator is returned
// which can be used to iterate over all keys in the query result set.
// However, if the number of keys in the query result set is greater than the
// totalQueryLimit (defined in core.yaml), this iterator cannot be used
// to fetch all keys in the query result set (results will be limited by
// the totalQueryLimit).
// The query is NOT re-executed during validation phase, phantom reads are
// not detected. That is, other committed transactions may have added,
// updated, or removed keys that impact the result set, and this would not
// be detected at validation/commit time.  Applications susceptible to this
// should therefore not use GetQueryResult as part of transactions that update
// ledger, and should limit use to read-only chaincode operations.
func (s *BatchState) GetQueryResult(query string) (shim.StateQueryIteratorInterface, error) {
	panic("not implemented") // TODO: Implement
}

// GetQueryResultWithPagination performs a "rich" query against a state database.
// It is only supported for state databases that support rich query,
// e.g., CouchDB. The query string is in the native syntax
// of the underlying state database. An iterator is returned
// which can be used to iterate over keys in the query result set.
// When an empty string is passed as a value to the bookmark argument, the returned
// iterator can be used to fetch the first `pageSize` of query results.
// When the bookmark is a non-emptry string, the iterator can be used to fetch
// the first `pageSize` keys between the bookmark and the last key in the query result.
// Note that only the bookmark present in a prior page of query results (ResponseMetadata)
// can be used as a value to the bookmark argument. Otherwise, an empty string
// must be passed as bookmark.
// This call is only supported in a read only transaction.
func (s *BatchState) GetQueryResultWithPagination(query string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *pb.QueryResponseMetadata, error) {
	panic("not implemented") // TODO: Implement
}

// GetHistoryForKey returns a history of key values across time.
// For each historic key update, the historic value and associated
// transaction id and timestamp are returned. The timestamp is the
// timestamp provided by the client in the proposal header.
// GetHistoryForKey requires peer configuration
// core.ledger.history.enableHistoryDatabase to be true.
// The query is NOT re-executed during validation phase, phantom reads are
// not detected. That is, other committed transactions may have updated
// the key concurrently, impacting the result set, and this would not be
// detected at validation/commit time. Applications susceptible to this
// should therefore not use GetHistoryForKey as part of transactions that
// update ledger, and should limit use to read-only chaincode operations.
func (s *BatchState) GetHistoryForKey(key string) (shim.HistoryQueryIteratorInterface, error) {
	panic("not implemented") // TODO: Implement
}

// GetPrivateData returns the value of the specified `key` from the specified
// `collection`. Note that GetPrivateData doesn't read data from the
// private writeset, which has not been committed to the `collection`. In
// other words, GetPrivateData doesn't consider data modified by PutPrivateData
// that has not been committed.
func (s *BatchState) GetPrivateData(collection string, key string) ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

// GetPrivateDataHash returns the hash of the value of the specified `key` from the specified
// `collection`
func (s *BatchState) GetPrivateDataHash(collection string, key string) ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

// PutPrivateData puts the specified `key` and `value` into the transaction's
// private writeset. Note that only hash of the private writeset goes into the
// transaction proposal response (which is sent to the client who issued the
// transaction) and the actual private writeset gets temporarily stored in a
// transient store. PutPrivateData doesn't effect the `collection` until the
// transaction is validated and successfully committed. Simple keys must not
// be an empty string and must not start with a null character (0x00) in order
// to avoid range query collisions with composite keys, which internally get
// prefixed with 0x00 as composite key namespace. In addition, if using
// CouchDB, keys can only contain valid UTF-8 strings and cannot begin with an
// an underscore ("_").
func (s *BatchState) PutPrivateData(collection string, key string, value []byte) error {
	panic("not implemented") // TODO: Implement
}

// DelPrivateData records the specified `key` to be deleted in the private writeset
// of the transaction. Note that only hash of the private writeset goes into the
// transaction proposal response (which is sent to the client who issued the
// transaction) and the actual private writeset gets temporarily stored in a
// transient store. The `key` and its value will be deleted from the collection
// when the transaction is validated and successfully committed.
func (s *BatchState) DelPrivateData(collection string, key string) error {
	panic("not implemented") // TODO: Implement
}

// SetPrivateDataValidationParameter sets the key-level endorsement policy
// for the private data specified by `key`.
func (s *BatchState) SetPrivateDataValidationParameter(collection string, key string, ep []byte) error {
	panic("not implemented") // TODO: Implement
}

// GetPrivateDataValidationParameter retrieves the key-level endorsement
// policy for the private data specified by `key`. Note that this introduces
// a read dependency on `key` in the transaction's readset.
func (s *BatchState) GetPrivateDataValidationParameter(collection string, key string) ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

// GetPrivateDataByRange returns a range iterator over a set of keys in a
// given private collection. The iterator can be used to iterate over all keys
// between the startKey (inclusive) and endKey (exclusive).
// The keys are returned by the iterator in lexical order. Note
// that startKey and endKey can be empty string, which implies unbounded range
// query on start or end.
// Call Close() on the returned StateQueryIteratorInterface object when done.
// The query is re-executed during validation phase to ensure result set
// has not changed since transaction endorsement (phantom reads detected).
func (s *BatchState) GetPrivateDataByRange(collection string, startKey string, endKey string) (shim.StateQueryIteratorInterface, error) {
	panic("not implemented") // TODO: Implement
}

// GetPrivateDataByPartialCompositeKey queries the state in a given private
// collection based on a given partial composite key. This function returns
// an iterator which can be used to iterate over all composite keys whose prefix
// matches the given partial composite key. The `objectType` and attributes are
// expected to have only valid utf8 strings and should not contain
// U+0000 (nil byte) and U+10FFFF (biggest and unallocated code point).
// See related functions SplitCompositeKey and CreateCompositeKey.
// Call Close() on the returned StateQueryIteratorInterface object when done.
// The query is re-executed during validation phase to ensure result set
// has not changed since transaction endorsement (phantom reads detected).
func (s *BatchState) GetPrivateDataByPartialCompositeKey(collection string, objectType string, keys []string) (shim.StateQueryIteratorInterface, error) {
	panic("not implemented") // TODO: Implement
}

// GetPrivateDataQueryResult performs a "rich" query against a given private
// collection. It is only supported for state databases that support rich query,
// e.g.CouchDB. The query string is in the native syntax
// of the underlying state database. An iterator is returned
// which can be used to iterate (next) over the query result set.
// The query is NOT re-executed during validation phase, phantom reads are
// not detected. That is, other committed transactions may have added,
// updated, or removed keys that impact the result set, and this would not
// be detected at validation/commit time.  Applications susceptible to this
// should therefore not use GetPrivateDataQueryResult as part of transactions that update
// ledger, and should limit use to read-only chaincode operations.
func (s *BatchState) GetPrivateDataQueryResult(collection string, query string) (shim.StateQueryIteratorInterface, error) {
	panic("not implemented") // TODO: Implement
}

// GetCreator returns `SignatureHeader.Creator` (e.g. an identity)
// of the `SignedProposal`. This is the identity of the agent (or user)
// submitting the transaction.
func (s *BatchState) GetCreator() ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

// GetTransient returns the `ChaincodeProposalPayload.Transient` field.
// It is a map that contains data (e.g. cryptographic material)
// that might be used to implement some form of application-level
// confidentiality. The contents of this field, as prescribed by
// `ChaincodeProposalPayload`, are supposed to always
// be omitted from the transaction and excluded from the ledger.
func (s *BatchState) GetTransient() (map[string][]byte, error) {
	panic("not implemented") // TODO: Implement
}

// GetBinding returns the transaction binding, which is used to enforce a
// link between application data (like those stored in the transient field
// above) to the proposal itself. This is useful to avoid possible replay
// attacks.
func (s *BatchState) GetBinding() ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

// GetDecorations returns additional data (if applicable) about the proposal
// that originated from the peer. This data is set by the decorators of the
// peer, which append or mutate the chaincode input passed to the chaincode.
func (s *BatchState) GetDecorations() map[string][]byte {
	panic("not implemented") // TODO: Implement
}

// GetSignedProposal returns the SignedProposal object, which contains all
// data elements part of a transaction proposal.
func (s *BatchState) GetSignedProposal() (*pb.SignedProposal, error) {
	panic("not implemented") // TODO: Implement
}

// GetTxTimestamp returns the timestamp when the transaction was created. This
// is taken from the transaction ChannelHeader, therefore it will indicate the
// client's timestamp and will have the same value across all endorsers.
func (s *BatchState) GetTxTimestamp() (*timestamp.Timestamp, error) {
	panic("not implemented") // TODO: Implement
}

// SetEvent allows the chaincode to set an event on the response to the
// proposal to be included as part of a transaction. The event will be
// available within the transaction in the committed block regardless of the
// validity of the transaction.
func (s *BatchState) SetEvent(name string, payload []byte) error {
	panic("not implemented") // TODO: Implement
}
