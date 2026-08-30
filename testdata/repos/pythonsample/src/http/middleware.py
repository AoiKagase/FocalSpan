from ..auth.token_service import TokenService

def authorize(service: TokenService, token):
    return service.validate_token(token)
