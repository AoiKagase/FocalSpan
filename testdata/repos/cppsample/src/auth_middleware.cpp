#include "auth/token_service.hpp"

namespace app::http {
bool Authenticate(app::auth::TokenService& service, const char* token) {
    return service.ValidateToken(token);
}
}

