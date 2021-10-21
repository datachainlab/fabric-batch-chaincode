# Fabric-Batch-Chaincode

Fabric-Batch-Chaincode (FBC) is a library that enables batch transactions in chaincode without additional trusted systems.

## Overview

In FBC, one unit of processing is called `Msg`, and it provides SubmitMsg function to submit it to the ledger, and Commit function to execute the submitted `Msg`s.

Each function performs the following operations:

- SubmitMsg(msg: Msg, current_time: int64)
  - SubmitMsg is a function that takes a `Msg` and timestamp `current_time` arguments
  - The validity of `current_time` is explained in [Validation for Current Time](#validation-for-current-time).
  - Validate that there is no commit after `current_time`.
  - Msg is stored in association with a key that contains `current_time` as prefix.

- Commit(commit_time: int64)
  - Commit is a function that takes a timestamp `commit_time` argument.
  - The `commit_time` should be less than the "Current Time Window" described later.
  - Get `Msg`s between the last committed time and `commit_time`.
  - Execute these Msgs, and save a commit log with `commit_time` as key prefix.

It also keeps the following states:

- last committed time: "lct" => {timestamp}
- msg: "msg/{timestamp}{TxID}"
- commit log: "commit/{timestamp}"

## Validation for Current Time

In fabric, no timestamp indicates the current time that is agreed between peers. Of course, it can get a current time in the chaincode, but this is based on the local clock and will be a different value for each peer.

FBC defines "Current Time" as a range defined by a parameter `TimeGapAllowance` based on the local clock of endorser peers.

## Correctness

FBC satisfies the following three properties:

(1) All Msgs will eventually be submitted
  - Insert Msg with `current_time` as key prefix in `SubmitMsg`
  - `Commit` can only be performed at a time older than `Current Time`

The above points guarantee that a Msg will eventually be submitted if a submitter can get a valid current time.

(2) There is no commit log newer than the time of the uncommitted Msg
  - Verify that there is no commit log newer than the current time with `GetStateByRange(current_time, ∞)`

This will recheck for the existence of the commit logs during the validation phase, and if there is a commit log with a time newer than `current_time`, the tx that contains the `SubmitMsg` will fail. Therefore, (2) is guaranteed.

(3) All submitted Msgs will eventually be committed
  - `commit_time` is always newer than previous `commit_time`
  - Timestamp of a Msg to be committed is within the range between the previous time and `commit_time`.

The above points and (2) guarantees (3) under the assumption that an actor performs `Commit`.

## Maintainers

- [Jun Kimura](https://github.com/bluele)
