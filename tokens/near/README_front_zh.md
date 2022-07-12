
## near
合约仓库： https://github.com/anyswap/near-contract 

## sdk
https://docs.near.org/docs/develop/front-end/near-api-js

## api
https://docs.near.org/docs/api/rpc

## get_tokenInfo
>1)tokenInfo 
```shell
# nep141合约
code: https://github.com/near/near-sdk-rs/blob/master/near-contract-standards/src/fungible_token/metadata.rs
method: fn ft_metadata(&self) -> FungibleTokenMetadata
```
>2)underlying
```shell
code: https://github.com/anyswap/near-contract/blob/main/anytoken/src/lib.rs
method: pub fn underlying(&self)->AccountId
```
>3)native balance
```shell
{
  "jsonrpc": "2.0",
  "id": "dontcare",
  "method": "query",
  "params": {
    "request_type": "view_account",
    "finality": "final",
    "account_id": "nearkat.testnet"
  }
}
```
>4)token balance
```shell
code: https://github.com/near/near-sdk-rs/blob/master/near-contract-standards/src/fungible_token/core.rs
method: fn ft_balance_of(&self, account_id: AccountId) -> U128
```
>5)totalSupply
```shell
code: https://github.com/near/near-sdk-rs/blob/master/near-contract-standards/src/fungible_token/core.rs
method: fn ft_balance_of(&self, account_id: AccountId) -> U128;
```
>6)storage deposit
```shell
code: https://github.com/near/near-sdk-rs/blob/master/near-contract-standards/src/fungible_token/storage_impl.rs
method: fn storage_deposit(
        &mut self,
        account_id: Option<AccountId>,
        registration_only: Option<bool>,
    ) -> StorageBalance
```
>7)balance of storage deposit
```shell
code: https://github.com/near/near-sdk-rs/blob/master/near-contract-standards/src/fungible_token/storage_impl.rs
method: fn storage_balance_of(&self, account_id: AccountId) -> Option<StorageBalance>
```
