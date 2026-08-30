Imports System
Imports System.Threading.Tasks
Namespace Demo
Partial Public Class MainWindow
    Inherits WindowBase
    Implements IAuthorizer
    Private Sub Button_Click(sender As Object, e As EventArgs) Handles LoginButton.Click
        AddHandler LoginButton.Click, AddressOf Button_Click
        ValidateTokenAsync("guest")
    End Sub
    Public Async Function ValidateTokenAsync(token As String) As Task(Of Boolean)
        Return Await ValidateToken(token)
    End Function
    Public Function ValidateToken(token As String) As Boolean
        Return Not String.IsNullOrEmpty(token)
    End Function
End Class
End Namespace
