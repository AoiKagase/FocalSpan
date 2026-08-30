use focalspan_rust_sample::auth::token_service::TokenService;

#[tokio::test]
async fn expired_token_is_rejected() {
    let service = TokenService::default();
    assert!(!service.validate_token("expired"));
}
