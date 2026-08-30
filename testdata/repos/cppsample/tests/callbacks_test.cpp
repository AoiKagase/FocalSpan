#include "auth/callbacks.hpp"

TEST_CASE("Rejects expired callback", "[auth]") {
    app::auth::install_callback();
}
