# Key Management CLI Design

## Objective
Provide a secure way to store and lease out sensitive keys (API tokens, secrets) via the Dialtone CLI.

## CLI Usage

### 1. Store a Key
Encrypts the value using a password-derived key and stores it.
```shell
./dialtone.sh key add <key-name> <key-value> <password>
```

### 2. Lease a Key (Retrieve)
Decrypts and outputs the key if the provided password is correct.
```shell
./dialtone.sh key <key-name> <password>
```

### 3. List Keys
Lists the names of all stored keys (does not show values).
```shell
./dialtone.sh key list
```

### 4. Remove a Key
Deletes the encrypted key from storage.
```shell
./dialtone.sh key rm <key-name>
```

## Security Strategy
- **Encryption**: AES-256-GCM (Authenticated Encryption).
- **Key Derivation**: PBKDF2 with SHA-256 or Argon2 to derive the 256-bit encryption key from the password.
- **Storage**: Keys are stored in a new `keys` table within `src/tickets/tickets.duckdb`.

## Examples

```shell
# Adding a Stripe API key
./dialtone.sh key add stripe "sk_test_..." "mypassword123"

# Leasing the key for use in a script
STRIPE_KEY=$(./dialtone.sh key stripe "mypassword123")
```
