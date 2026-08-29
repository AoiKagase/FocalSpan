#include "../include/config.h"

int legacy_validate_token(const char* token);

int legacy_validate_token(const char* token) {
    return token != 0 && token[0] != '\0';
}

