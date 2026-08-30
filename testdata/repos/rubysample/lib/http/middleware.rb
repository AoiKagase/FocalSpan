require_relative "../auth/token_service"

module FocalSpan
  class AuthMiddleware
    def authorize(request)
      TokenService.build(request.token).validate_token(request.token)
    end
  end
end
