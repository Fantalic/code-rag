//! Inventory module. Mirrors a service-layer struct with methods,
//! imports the standard library explicitly, and uses an error set so
//! the analyser sees an enum-like set of error names.

const std = @import("std");
const Order = @import("order.zig").Order;

pub const InventoryError = error{
    DuplicateSku,
    NegativeQuantity,
};

pub const Inventory = struct {
    items: std.ArrayList(Order),

    pub fn init(allocator: std.mem.Allocator) Inventory {
        return .{ .items = std.ArrayList(Order).init(allocator) };
    }

    pub fn deinit(self: *Inventory) void {
        self.items.deinit();
    }

    pub fn add(self: *Inventory, order: Order) InventoryError!void {
        if (order.total < 0) return InventoryError.NegativeQuantity;
        try self.items.append(order);
    }

    pub fn has(self: Inventory, id: u32) bool {
        for (self.items.items) |item| {
            if (item.id == id) return true;
        }
        return false;
    }

    pub fn count(self: Inventory) usize {
        return self.items.items.len;
    }
};

test "inventory starts empty" {
    const allocator = std.testing.allocator;
    var inv = Inventory.init(allocator);
    defer inv.deinit();
    try std.testing.expectEqual(@as(usize, 0), inv.count());
}
