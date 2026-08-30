require "minitest/autorun"
require_relative "../lib/auth/token_service"

class TokenServiceTest < Minitest::Test
  def test_expired_token_is_rejected
    refute FocalSpan::TokenService.build("expired").validate_token("expired")
  end
end
