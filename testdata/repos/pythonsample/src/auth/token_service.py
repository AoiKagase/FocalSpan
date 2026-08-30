from .types import TokenClaims

class TokenService:
    def validate_token(self, token: TokenClaims) -> bool:
        return bool(token.value)

    @classmethod
    async def build(cls, token: TokenClaims):
        return cls()

def validate_token(token: TokenClaims) -> bool:
    return TokenService().validate_token(token)
