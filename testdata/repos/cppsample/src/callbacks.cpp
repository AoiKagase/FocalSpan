#include "auth/callbacks.hpp"

namespace app::auth {
struct CallbackConfig { int timeout; };
CallbackConfig config = {.timeout = 30};

concept Positive = requires(int value) { value > 0; };

void handler(int value) { (void)value; }
void register_callback(TokenCallback callback) { callback(1); }
void install_callback() { register_callback(handler); }
}
