Imports Demo
Namespace Demo.Tests
Public Class AuthTests
    Public Sub TestExpiredTokenIsRejected()
        Dim service As New MainWindow
        Assert.False(service.ValidateToken("expired"))
    End Sub
End Class
End Namespace
