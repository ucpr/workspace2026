const std = @import("std");
const sdk = @import("opentelemetry-sdk");

const metrics_sdk = sdk.metrics;

const meter_name = "workspace2026/zig_otel_sdk_build";

pub fn run(writer: anytype) !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer std.debug.assert(gpa.deinit() == .ok);

    try writeExample(gpa.allocator(), writer);
}

fn writeExample(allocator: std.mem.Allocator, writer: anytype) !void {
    const meter_provider = try metrics_sdk.MeterProvider.init(allocator);
    defer meter_provider.shutdown();

    const exporter = try metrics_sdk.MetricExporter.InMemory(allocator, null, null);
    defer exporter.in_memory.deinit();

    const reader = try metrics_sdk.MetricReader.init(allocator, exporter.exporter);
    defer reader.shutdown();

    try meter_provider.addReader(reader);

    const meter = try meter_provider.getMeter(.{
        .name = meter_name,
        .version = "0.1.0",
    });

    const request_counter = try meter.createCounter(u64, .{
        .name = "http.server.requests",
        .description = "Total number of HTTP requests handled by the sample app",
        .unit = "1",
    });
    const request_duration = try meter.createHistogram(f64, .{
        .name = "http.server.duration",
        .description = "HTTP request latency recorded by the sample app",
        .unit = "ms",
    });

    const get = @as([]const u8, "GET");
    const post = @as([]const u8, "POST");
    const healthz = @as([]const u8, "/healthz");
    const orders = @as([]const u8, "/orders");

    try request_counter.add(1, .{ "method", get, "route", healthz, "status", @as(u64, 200) });
    try request_counter.add(1, .{ "method", get, "route", healthz, "status", @as(u64, 200) });
    try request_counter.add(1, .{ "method", post, "route", orders, "status", @as(u64, 500) });

    try request_duration.record(12.4, .{ "method", get, "route", healthz });
    try request_duration.record(18.7, .{ "method", get, "route", healthz });
    try request_duration.record(245.3, .{ "method", post, "route", orders });

    try reader.collect();

    const stored_metrics = try exporter.in_memory.fetch(allocator);
    defer {
        for (stored_metrics) |*metric| {
            metric.deinit(allocator);
        }
        allocator.free(stored_metrics);
    }

    try writer.print("OpenTelemetry metrics example\n", .{});
    try writer.print("meter={s}\n", .{meter_name});
    try writer.print("collected_metric_streams={d}\n\n", .{stored_metrics.len});

    for (stored_metrics) |metric| {
        try writer.print("metric {s} kind={s}", .{
            metric.instrumentOptions.name,
            @tagName(metric.instrumentKind),
        });
        if (metric.instrumentOptions.unit) |unit| {
            try writer.print(" unit={s}", .{unit});
        }
        try writer.writeAll("\n");

        if (metric.instrumentOptions.description) |description| {
            try writer.print("  description={s}\n", .{description});
        }

        switch (metric.data) {
            .int => |points| {
                for (points) |point| {
                    try writer.print("  point value={d}", .{point.value});
                    try writeAttributes(writer, point.attributes);
                    try writer.writeAll("\n");
                }
            },
            .double => |points| {
                for (points) |point| {
                    try writer.print("  point value={d}", .{point.value});
                    try writeAttributes(writer, point.attributes);
                    try writer.writeAll("\n");
                }
            },
            .histogram => |points| {
                for (points) |point| {
                    try writer.print("  histogram count={d}", .{point.value.count});
                    if (point.value.sum) |sum| {
                        try writer.print(" sum={d}", .{sum});
                    }
                    if (point.value.min) |min| {
                        try writer.print(" min={d}", .{min});
                    }
                    if (point.value.max) |max| {
                        try writer.print(" max={d}", .{max});
                    }
                    try writeAttributes(writer, point.attributes);
                    try writer.writeAll("\n");
                }
            },
            .exponential_histogram => |points| {
                for (points) |point| {
                    try writer.print("  exponential_histogram count={d}", .{point.value.count});
                    if (point.value.sum) |sum| {
                        try writer.print(" sum={d}", .{sum});
                    }
                    try writeAttributes(writer, point.attributes);
                    try writer.writeAll("\n");
                }
            },
        }

        try writer.writeAll("\n");
    }
}

fn writeAttributes(writer: anytype, attributes: anytype) !void {
    if (attributes) |attrs| {
        try writer.writeAll(" attrs={");
        for (attrs, 0..) |attr, index| {
            if (index > 0) {
                try writer.writeAll(", ");
            }
            try writer.print("{s}=", .{attr.key});
            try writeAttributeValue(writer, attr.value);
        }
        try writer.writeAll("}");
    }
}

fn writeAttributeValue(writer: anytype, value: sdk.AttributeValue) !void {
    switch (value) {
        .bool => |bool_value| try writer.print("{}", .{bool_value}),
        .string => |string_value| try writer.print("{s}", .{string_value}),
        .int => |int_value| try writer.print("{d}", .{int_value}),
        .double => |double_value| try writer.print("{d}", .{double_value}),
    }
}

test "metrics example emits counter and histogram output" {
    var output = std.ArrayList(u8).init(std.testing.allocator);
    defer output.deinit();

    try writeExample(std.testing.allocator, output.writer());

    try std.testing.expect(std.mem.indexOf(u8, output.items, "metric http.server.requests kind=Counter") != null);
    try std.testing.expect(std.mem.indexOf(u8, output.items, "metric http.server.duration kind=Histogram") != null);
    try std.testing.expect(std.mem.indexOf(u8, output.items, "point value=2") != null);
    try std.testing.expect(std.mem.indexOf(u8, output.items, "histogram count=2") != null);
}
