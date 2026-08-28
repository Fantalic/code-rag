//! Entry point. Wires up the inventory + service and exercises every
//! public call path so the analyser sees a complete caller -> callee
//! graph when scanning the project.

const std = @import("std");
const Order = @import("order.zig").Order;
const Status = @import("order.zig").Status;
const Inventory = @import("inventory.zig").Inventory;
const OrderService = @import("order_service.zig").OrderService;

/// Application configuration carried around by main.
pub const Config = struct {
    verbose: bool = false,
};

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var inv = Inventory.init(allocator);
    defer inv.deinit();

    const svc = OrderService.init(&inv);
    _ = try svc.place(.{ .id = 1, .customer = "Ada", .total = 19.95, .status = Status.pending });
    _ = try svc.place(.{ .id = 2, .customer = "Bob", .total = 5.00, .status = Status.paid });

    std.debug.print("size={d}\n", .{svc.size()});
    std.debug.print("highest={d}\n", .{OrderService.highestId(inv)});
}

/// Free helper: a point on a 2-D plane. Demonstrates a struct with
/// only fields and a const declaration alongside it.
pub const Point = struct {
    x: f32,
    y: f32,
};

pub const origin = Point{ .x = 0.0, .y = 0.0 };

test "origin is at (0, 0)" {
    try std.testing.expectEqual(@as(f32, 0.0), origin.x);
    try std.testing.expectEqual(@as(f32, 0.0), origin.y);
}
