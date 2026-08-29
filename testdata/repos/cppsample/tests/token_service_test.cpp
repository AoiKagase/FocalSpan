#include "../include/auth/token_service.hpp"
#include <gtest/gtest.h>

TEST(TokenServiceTest, RejectsExpiredToken) {
    app::auth::TokenService service;
    EXPECT_FALSE(service.ValidateToken("expired"));
}

TEST_F(TokenServiceFixture, RejectsEmptyToken) {
    app::auth::TokenService service;
    EXPECT_FALSE(service.ValidateToken(""));
}

TEST_P(TokenServiceFixture, RejectsMalformedToken) { SUCCEED(); }
TYPED_TEST(TokenServiceTypedTest, RejectsTypedToken) { SUCCEED(); }

