public class Point { public int X; public int Y; }
public class App {
    public App() { }
    public void Run() {
        var p = new Point();
        p.GetType();
        this.Foo();
        BadStatic();
    }
    private void Foo() => Console.WriteLine("hi");
}
