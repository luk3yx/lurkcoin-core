# TODO

 - Fix exchange rate calculations.
 - Make the limits on the size of V2 requests more strict.
 - Allow a client-provided identifier in `/v3/pay` to prevent double-ups.
 - Rate limit failed login attempts (on both API endpoints and the admin pages).
 - Add `/v3/regenerate_token`. To ensure atomicity, lurkcoin will accept both
    tokens until the new token is used.
 - Don't use big.Float when converting Currency objects from strings.
 - Don't escape HTML tags in the returned JSON (possibly).
 - Add a way to request account creation and webhook URLs.
 - Add PBKDF2 for admin pages password hashes.
 - Federation.
