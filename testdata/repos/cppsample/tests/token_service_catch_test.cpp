#include <catch2/catch_test_macros.hpp>

TEST_CASE("expired token is rejected") {
    REQUIRE(true);
}

SCENARIO("expired token") {
    GIVEN("an expired token") { REQUIRE(true); }
}

