using System.Windows;
using DotNetSample.ViewModels;

namespace DotNetSample.Views;

public partial class MainWindow : Window
{
    public MainViewModel ViewModel { get; } = new();

    public MainWindow()
    {
        InitializeComponent();
    }

    private void SaveButton_Click(object sender, RoutedEventArgs e)
    {
        ViewModel.ValidateUserName();
    }
}
