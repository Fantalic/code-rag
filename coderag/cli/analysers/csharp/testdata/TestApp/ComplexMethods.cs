using System;
using System.Threading;
using System.Threading.Tasks;

namespace TestApp
{
    class ComplexMethods
    {
        public bool TryParse(string input, out int result)
        {
            return int.TryParse(input, out result);
        }

        public void Swap(ref int a, ref int b)
        {
            int tmp = a;
            a = b;
            b = tmp;
        }

        public void Log(params string[] messages)
        {
            foreach (var m in messages) Console.WriteLine(m);
        }

        public int Process(Func<int, bool> predicate, int seed)
        {
            return predicate(seed) ? seed : 0;
        }

        public void Send(string to, int priority = 0, bool ccSender = true)
        {
        }

        public void Save([NotNull] string key, [NotNull] string value)
        {
        }

        public static int ExtendTo(this string s, int times)
        {
            int n = 0;
            for (int i = 0; i < times; i++) n += s.Length;
            return n;
        }

        public async Task<Order> GetAsync(int id, CancellationToken ct = default)
        {
            await Task.Delay(1, ct);
            return new Order { Id = id };
        }

        public T Find<T>(T item, Func<T, bool> match) where T : class
        {
            return match(item) ? item : null;
        }
    }
}
