#include "auth/token_service.hpp"
#include <memory>

std::unique_ptr<app::auth::TokenService> BuildTokenService() {
    return std::make_unique<app::auth::TokenService>();
}

