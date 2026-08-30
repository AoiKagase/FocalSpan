#include <amxmodx>
#include "auth.inc"

new g_token[64];
const MAX_ATTEMPTS = 3;

stock bool:validate_token(const value[]) {
  return value[0] != EOS;
}

public plugin_init() {
  register_plugin("Auth", "1.0", "FocalSpan");
  register_clcmd("say /login", "cmd_login");
}

public cmd_login(id) {
  if (validate_token(g_token)) {
    set_task(1.0, "finish_login", id);
  }
}

public finish_login(id) {
  client_print(id, print_chat, "ok");
}
