//! Domain type: a customer order. Exercises struct, fields, enum,
//! error set, and a free helper function. Consumed by main.zig and
//! order_service.zig to give the cross-file call graph something
//! non-trivial to resolve.

const std = @import("std");

pub const Status = enum {
    pending,
    paid,
    shipped,
    delivered,
    cancelled,
};

pub const OrderError = error{
    NotFound,
    InvalidQuantity,
    OutOfStock,
};

pub const Order = struct {
    id: u32,
    customer: []const u8,
    total: f64,
    status: Status,
};

/// Returns the price as a formatted string.
pub fn totalPrice(order: Order, writer: anytype) !void {
    try writer.print("{d:.2}", .{order.total});
}

/// Validates an order's basic invariants. Pure helper, no I/O.
pub fn validate(order: Order) OrderError!void {
    if (order.id == 0) return OrderError.NotFound;
    if (order.total < 0) return OrderError.InvalidQuantity;
}

test "status transitions cover all states" {
    const states = [_]Status{ .pending, .paid, .shipped, .delivered, .cancelled };
    try std.testing.expectEqual(@as(usize, 5), states.len);
}
