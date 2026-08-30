VERSION 1.0 CLASS
Attribute VB_Name = "AuthService"
Option Explicit
Implements IAuthorizer
Public Event Authorized(user As String)
Public Property Get CurrentUser() As String
    CurrentUser = mCurrentUser
End Property
Public Function ValidateToken(ByVal token As String) As Boolean
    ValidateToken = Len(token) > 0
End Function
Public Sub Login(ByVal token As String)
    If ValidateToken(token) Then RaiseEvent Authorized(token)
End Sub
