namespace Demo;
public partial class Service {
    public required string DisplayName { get; init; }
    public T Echo<T>(T value) { return value; }
    public void Save() { }
    public void Run() => Save();
    public void Foreign() => external.Save();
}
