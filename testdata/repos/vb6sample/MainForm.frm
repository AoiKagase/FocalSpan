VERSION 5.00
Begin VB.Form MainForm
   Caption         =   "Authentication"
   ClientHeight    =   3000
   ClientWidth     =   5000
End
Attribute VB_Name = "MainForm"
Option Explicit
Private WithEvents service As AuthService

Private Sub Form_Load()
    service.Login "guest"
End Sub

Private Sub service_Authorized(ByVal user As String)
    Caption = user
End Sub
