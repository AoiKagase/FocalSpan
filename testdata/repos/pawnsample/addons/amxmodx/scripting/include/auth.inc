#if defined _auth_included
  #endinput
#endif
#define _auth_included

native get_user_authid(id, buffer[], length);
forward bool:validate_token(const value[]);
