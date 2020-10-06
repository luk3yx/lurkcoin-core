# lurkcoinV3 API documentation

lurkcoin provides an API for servers and other integrations.

In version 3 of the API, JSON is recommended for requests and used for all
responses. If you cannot decode JSON, you should probably use the legacy
[lurkcoinV2 API](https://gist.github.com/luk3yx/8028cedb3bfb282d9ba3f2d1c7871231).
There are currently no plans to remove the older API.

All API endpoints require an `Authorization` header with the
[`Basic`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication#Basic_authentication_scheme)
authentication scheme. If the username somehow happens to contain colons (`:`),
these can be replaced with underscores (`_`).

*With Python's requests library, you can simply add
`auth=('username', 'token')` as a keyword argument.*

# Response format

All responses will be a JSON object containing a `success` boolean. If lurkcoin
encounters errors processing your request, this will be false and an error code
will be added to the response. Otherwise, the response data (if any) will be in
the `result` key.

The `X-Force-OK` header can be set to `true` to force a `200 OK` reply even
when an error occurs.

### Examples

```json
{
    "success": true,
    "result": 1234.56
}
```

```json
{
    "success": false,
    "error": "ERR_CANNOTAFFORD",
    "message": "You cannot afford to do that!"
}
```

### Globally raised errors

The following errors may be raised on any API endpoint:
 - `ERR_INVALIDLOGIN` when either `username` or `token` is invalid.
 - `ERR_INVALIDREQUEST` when required parameters are missing or are an invalid
    type.
 - `ERR_INTERNALERROR` when something really nasty happens.

# API endpoints

## GET `/v3/summary`

Returns an "account summary".

This endpoint returns a JSON-formatted object with the following items:
 - `uid`: An internal UID for the user, this is probably the username with
    only the following characters: `[A-Za-z0-9_]+`.
 - `name`: The username for the user. This is used in [transaction objects].
 - `bal`: A number with the user's current balance.
 - `balance`: The balance formatted as a string (if `bal` is `1.23`,
    `balance` will be `¤1.23` or similar).
 - `history`: A list with the 10 most recent [transaction objects].
 - `interest_rate`: The current interest rate.
 - `target_balance`: The server's target balance. This will be
    `0` if the server's local currency is equal to lurkcoin.

## POST `/v3/pay`

Sends a payment to a user. This will return the transaction object (see
`/v3/history`) on success. This can optionally be used to generate transaction
IDs for local transactions.

Parameters:
 - `source`: The user who is sending the transaction.
 - `target`: The target user to pay.
 - `target_server`: The server to pay the user on.
 - `amount`: The amount to pay the user.
 - `local_currency`: If `true`, lurkcoin will calculate the
    local server's exchange rate before processing the transaction.

Errors raised:
 - `ERR_SERVERNOTFOUND` when `target_server` doesn't exist.
 - `ERR_INVALIDAMOUNT` when the amount is invalid
 - `ERR_CANNOTPAYNOTHING` when the amount (after exchange rate calculations)
    is ¤0.00.
 - `ERR_CANNOTAFFORD` when your balance is lower than the amount sent (in
    lurkcoins).

## GET `/v3/balance`

Returns your account balance as a number.

## GET `/v3/history`

Returns a list of [transaction objects].

## POST `/v3/exchange_rates`

Gets the exchange rate for the specified server. Will return a number.

Parameters:
 - `source` *(optional)*: The server the money is hypothetically coming from.
 - `target` *(optional)*: The server the money is hypothetically going to.
 - `amount`: The amount of money being transferred.

Errors raised:
 - `ERR_SOURCESERVERNOTFOUND` when `source` doesn't exist.
 - `ERR_TARGETSERVERNOTFOUND` when `target` doesn't exist.
 - `ERR_INVALIDAMOUNT` when `amount` is invalid.

## GET `/v3/pending_transactions`

Returns a JSON-formatted list of unprocessed [transaction objects]. Note that
the order of this list is not guaranteed to be the same between API calls.

## POST `/v3/acknowledge_transactions`

Marks transactions as processed. Invalid or already processed transaction IDs
will be silently ignored.

**Please cache processed transaction IDs until this request has succeeded to
stop transactions being applied twice.**

Parameters:
 - `transactions`: A list of transaction IDs that have been processed.

## POST `/v3/reject_transactions`

Marks transactions as rejected, for example when one was sent to a non-existent
user. Invalid or already processed transaction IDs will be silently ignored.

*If a transaction gets marked as rejected, the target user (if any) must not
receive the transaction as the transaction may be reverted.*

Parameters:
 - `transactions`: A list of transaction IDs that have been rejected.

## GET `/v3/target_balance`

Gets the target balance. This will be `0` if the server's currency is equal to
lurkcoin.

## PUT `/v3/target_balance`

Sets the server's current balance.

Target balances are used with calculating exchange rates, if the server's
balance is lower than this target balance the currency will be less valuable
than lurkcoin.

**Please do not set ridiculously high/low target balances without a good reason
for doing so.** This API endpoint may have a maximum/minimum target balance
added in the future.

Set the target balance to `0` if the server's currency should have a 1:1
exchange rate with lurkcoin (or if the server's local currency *is* lurkcoin).

Parameters:
 - `target_balance`: The new target balance.

Errors raised:
 - `ERR_INVALIDAMOUNT`: Invalid target balance.

## GET `/v3/webhook_url`

Gets the server's webhook URL, or `null` if webhooks are not enabled. Every
time a transaction gets sent to the server a POST request will be sent to this
URL similar to the below example.

```
POST /lurkcoin HTTP/1.1
User-Agent: lurkcoin/3.0
Content-Length: 14
Content-Type: application/json

{"version": 0}
```

The request does not contain transaction information because there is currently
no reliable way to validate that the request has indeed originated from
lurkcoin.

# Alternate API endpoints

All `GET`-based endpoints also accept `POST`.

## POST `/v3/set_target_balance`

Equivalent to sending a PUT to `/v3/target_balance`. Can be used if you can't
or don't want to send `PUT` requests.

# Transaction objects

[transaction objects]: #transaction-objects

Transaction objects are defined as follows:

```js
{
    // The transaction ID
    "id": "T5E1816DE-9ACB0442",

    // The user who sent this transaction and the server they are on.
    "source": "sourceuser",
    "source_server": "sourceserver",

    // The user who has received this transaction and their server.
    "target": "targetuser",
    "target_server": "targetserver",

    // The amount sent in lurkcoins.
    "amount": 851.80,

    // The amount in the sending server's local currency.
    "sent_amount": 123.45,

    // The amount in the receiving server's local currency.
    "received_amount": 1650.52

    // The time the transaction was sent in seconds since the UNIX epoch.
    "time": 1578637022,

    // If this is false, the transaction will not be reverted if it gets
    // rejected by the receiving server.
    "revertable": true,
}
```

Extra items must be ignored by the client as these may be used in the future
to add more features.
