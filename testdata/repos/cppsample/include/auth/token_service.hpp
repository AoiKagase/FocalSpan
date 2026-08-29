#pragma once
#include "auth/token_validator.hpp"
#include <string_view>

namespace app::auth {
class TokenService final : public ITokenValidator {
public:
    TokenService();
    ~TokenService() override;
    bool ValidateToken(std::string_view token) const override;
    bool IsExpired(std::string_view token) const;
};
}

