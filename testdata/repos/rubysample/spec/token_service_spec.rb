require "../lib/auth/token_service"

RSpec.describe FocalSpan::TokenService do
  it "accepts a live token" do
    FocalSpan::TokenService.build("live").validate_token("live")
  end
end
