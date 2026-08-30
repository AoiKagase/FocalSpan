#pragma once

namespace app::auth {
using TokenCallback = void (*)(int);
void register_callback(TokenCallback callback);
void install_callback();
}
