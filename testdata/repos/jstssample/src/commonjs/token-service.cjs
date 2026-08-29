const { validateToken } = require("../auth/token-validator");
function checkToken(token) { return validateToken(token); }
module.exports = { validateToken, checkToken };

