namespace DotNetSample.Forms;

partial class MainForm
{
    private Button saveButton;

    private void InitializeComponent()
    {
        saveButton = new Button();
        saveButton.Click += SaveButton_Click;
        Load += MainForm_Load;
        Controls.Add(saveButton);
        resources.ApplyResources(saveButton, "saveButton");
    }

    private void SaveButton_Click(object sender, EventArgs e) { }
}
