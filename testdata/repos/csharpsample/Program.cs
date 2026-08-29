using App.Auth;

var service = new TokenService("program");
Console.WriteLine(service.ValidateToken(args[0]));

