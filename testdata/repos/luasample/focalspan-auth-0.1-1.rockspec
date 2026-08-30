package = "focalspan-auth"
version = "0.1-1"
source = { url = "https://example.invalid/focalspan-auth.tar.gz" }
build = {
  type = "builtin",
  modules = { ["auth.token_service"] = "lib/auth/token_service.lua" }
}
