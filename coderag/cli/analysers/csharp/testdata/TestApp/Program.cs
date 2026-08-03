using System;

namespace TestApp
{
    class Program
    {
        static void Main(string[] args)
        {
            var service = new OrderService();
            service.PlaceOrder(new Order { Id = 1 });
        }
    }
}
