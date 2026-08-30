local auth = require("auth.token_service")

function plugin_init()
  return auth:authorize({ token = "live" })
end
