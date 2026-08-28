//! Order service: orchestrates Order + Inventory. Exercises cross-file
//! calls (create -> Inventory.add, has, count) so the analyser can
//! demonstrate cross-file resolution end to end.

const std = @import("std");
const Order = @import("order.zig").Order;
const Status = @import("order.zig").Status;
const Inventory = @import("inventory.zig").Inventory;

pub const OrderService = struct {
    inventory: *Inventory,

    pub fn init(inventory: *Inventory) OrderService {
        return .{ .inventory = inventory };
    }

    /// Place a new order, recording it in inventory. Returns the
    /// assigned id or an error if validation fails.
    pub fn place(self: OrderService, order: Order) !u32 {
        try self.inventory.add(order);
        return order.id;
    }

    /// Look up whether an id is already known to the inventory.
    pub fn exists(self: OrderService, id: u32) bool {
        return self.inventory.has(id);
    }

    /// Total number of orders in the system.
    pub fn size(self: OrderService) usize {
        return self.inventory.count();
    }

    /// Free helper that doesn't take self: walks the inventory and
    /// returns the highest order id, or 0 when empty.
    pub fn highestId(inventory: Inventory) u32 {
        var max: u32 = 0;
        for (inventory.items.items) |item| {
            if (item.id > max) max = item.id;
        }
        return max;
    }
};

test "place assigns id and exists returns true" {
    const allocator = std.testing.allocator;
    var inv = Inventory.init(allocator);
    defer inv.deinit();

    const svc = OrderService.init(&inv);
    _ = try svc.place(.{ .id = 42, .customer = "Ada", .total = 9.99, .status = Status.pending });
    try std.testing.expect(svc.exists(42));
    try std.testing.expectEqual(@as(usize, 1), svc.size());
}
