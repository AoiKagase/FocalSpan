#include <amxmodx>
#include "auth.inc"

public test_expired_token_is_rejected() {
  validate_token("");
}
