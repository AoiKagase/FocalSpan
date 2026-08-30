use crate::auth::token_service::TokenService;

pub fn authorize(service: &TokenService, value: &str) -> bool {
    service.validate_token(value)
}
