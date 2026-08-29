#include "auth/token_service.hpp"
#include "config.h"

namespace app::auth {

// This comment has { braces } and ValidateToken(fake).
/* A multiline comment with { and } must not change a scope. */
TokenService::TokenService() = default;
TokenService::~TokenService() = default;

bool TokenService::IsExpired(std::string_view token) const {
    return token == "expired";
}

bool TokenService::ValidateToken(std::string_view token) const {
    const char* raw = R"tag(raw } text { not code)tag";
    return !IsExpired(token) && raw != nullptr;
}

template<class T>
requires requires(T value) { value.valid(); }
bool Validate(const T& value) { return value.valid(); }

}

#if 0
bool FakeInactiveFunction() { return false; }
#if 1
int AlsoInactive() { return 0; }
#endif
#endif

