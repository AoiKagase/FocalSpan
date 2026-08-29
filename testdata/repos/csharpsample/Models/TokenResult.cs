namespace App.Models;

public record TokenResult(bool Valid, string Reason);
public enum TokenState { Valid, Expired }
public delegate bool TokenPredicate(string token);

