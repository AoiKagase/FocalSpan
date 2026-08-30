pub mod auth;
pub mod http;

pub fn new_service() -> auth::token_service::TokenService {
    auth::token_service::TokenService::default()
}
