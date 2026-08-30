Attribute VB_Name = "AuthModule"
Option Explicit
Public Const DefaultRole As String = "user"
Public Sub StartAuthentication()
    Dim service As New AuthService
    service.Login "guest"
End Sub
