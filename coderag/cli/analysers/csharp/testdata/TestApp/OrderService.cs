using System;

namespace TestApp
{
    class OrderService
    {
        private Order _order;

        public OrderService()
        {
            _order = new Order();
        }

        public void PlaceOrder(Order order)
        {
            if (order == null)
            {
                throw new ArgumentNullException(nameof(order));
            }
            order.Total = CalculateTotal(order);
        }

        private decimal CalculateTotal(Order order)
        {
            return order.Total;
        }
    }
}
