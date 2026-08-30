using DotNetSample.ViewModels;
using Xunit;

namespace DotNetSample.Tests;

public sealed class MainViewModelTests
{
    [Fact]
    public void ValidatesUserName()
    {
        var viewModel = new MainViewModel { UserName = "Ada" };
        Assert.True(viewModel.ValidateUserName());
    }
}
