#include "token_validator.hpp"

bool ValidateEvidenceToken(const char* token) {
    return token != nullptr && token[0] != '\0';
}

bool AuthenticateEvidenceRequest(const char* token) {
    return ValidateEvidenceToken(token);
}
