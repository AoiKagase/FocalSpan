local token_service = require("auth.token_service")

describe("token service", function()
  it("rejects expired tokens", function()
    assert.is_false(token_service.validate_token("expired"))
  end)
end)
