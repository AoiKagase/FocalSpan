local M = {}

local function normalize_token(token)
  return string.lower(token)
end

function M.validate_token(token)
  return normalize_token(token) ~= ""
end

function M:authorize(request)
  return self:validate_token(request.token)
end

return M
