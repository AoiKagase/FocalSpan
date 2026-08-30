require_relative "token_validator"

module FocalSpan
  class TokenService < BaseService
    include TokenValidator
    TOKEN_KIND = :bearer
    attr_reader :token

    def initialize(token)
      @token = token
    end

    def validate_token(token)
      normalized = normalize_token(token)
      valid_token?(normalized)
    end

    def self.build(token)
      new(token)
    end

    define_method(:normalize_token) do |value|
      value.to_s.strip
    end
  end
end
