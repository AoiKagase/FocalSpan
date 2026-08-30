Imports System.Windows.Forms
Namespace Demo.Forms
Public Partial Class MainForm
    Public Event Authorized As EventHandler
    Public Property CurrentUser As String
    Private Sub Submit_Click(sender As Object, e As EventArgs) Handles SubmitButton.Click
        RaiseEvent Authorized(Me, e)
    End Sub
End Class
End Namespace
