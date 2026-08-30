using System;
using System.Windows.Forms;

namespace DotNetSample.Forms;

public partial class MainForm : Form
{
    public MainForm()
    {
        InitializeComponent();
    }

    private void MainForm_Load(object sender, EventArgs e)
    {
        Text = "Ready";
    }
}
