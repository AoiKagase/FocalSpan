namespace DotNetSample.ViewModels;

public sealed class MainViewModel
{
    public string UserName { get; set; } = "";

    public bool ValidateUserName()
    {
        return !string.IsNullOrWhiteSpace(UserName);
    }
}
