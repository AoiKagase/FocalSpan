#pragma once

namespace app::auth {
class ITokenValidator {
public:
    virtual ~ITokenValidator() = default;
    virtual bool ValidateToken(const char* token) const = 0;
};
}

