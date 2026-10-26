// huginn-sync-mac: one-way push of every Calendar.app calendar to a huginn server.
// Reads events via EventKit (local, iCloud, Google, Exchange — whatever Calendar.app has),
// sends one full snapshot per calendar to PUT /api/sync/calendars/:externalID.
// Config via env: HUGINN_URL, HUGINN_TOKEN (app password from Settings → Devices),
// optional HUGINN_PAST_DAYS (default 365), HUGINN_FUTURE_DAYS (default 730).
import EventKit
import Foundation

func env(_ k: String, _ def: String = "") -> String { ProcessInfo.processInfo.environment[k] ?? def }
func fail(_ msg: String) -> Never { FileHandle.standardError.write((msg + "\n").data(using: .utf8)!); exit(1) }

guard let base = URL(string: env("HUGINN_URL")), !env("HUGINN_TOKEN").isEmpty else {
    fail("set HUGINN_URL and HUGINN_TOKEN")
}
let token = env("HUGINN_TOKEN")
let pastDays = Double(env("HUGINN_PAST_DAYS", "365"))!
let futureDays = Double(env("HUGINN_FUTURE_DAYS", "730"))!

let store = EKEventStore()
let sem = DispatchSemaphore(value: 0)
var granted = false
if #available(macOS 14, *) {
    store.requestFullAccessToEvents { ok, _ in granted = ok; sem.signal() }
} else {
    store.requestAccess(to: .event) { ok, _ in granted = ok; sem.signal() }
}
sem.wait()
if !granted { fail("calendar access denied — allow it in System Settings → Privacy & Security → Calendars") }

let iso = ISO8601DateFormatter()
iso.formatOptions = [.withInternetDateTime]
let from = Date().addingTimeInterval(-pastDays * 86400)
let to = Date().addingTimeInterval(futureDays * 86400)

func hex(_ cg: CGColor?) -> String {
    guard let c = cg?.components, c.count >= 3 else { return "" }
    return String(format: "#%02x%02x%02x", Int(c[0] * 255), Int(c[1] * 255), Int(c[2] * 255))
}

// ponytail: recurring series are sent as expanded occurrences (uid = seriesUID_start),
// not as RRULE masters. Edited/deleted single occurrences come for free; cost is one
// row per occurrence in the window. Send RRULE masters if row count ever matters.
func snapshot(_ cal: EKCalendar) -> [String: Any] {
    let pred = store.predicateForEvents(withStart: from, end: to, calendars: [cal])
    let events: [[String: Any]] = store.events(matching: pred).map { e in
        let seriesUID = e.calendarItemExternalIdentifier ?? e.eventIdentifier ?? UUID().uuidString
        let uid = e.hasRecurrenceRules || e.isDetached ? "\(seriesUID)_\(Int(e.startDate.timeIntervalSince1970))" : seriesUID
        return [
            "uid": uid,
            "title": e.title ?? "",
            "description": e.notes ?? "",
            "location": e.location ?? "",
            "start": iso.string(from: e.startDate),
            "end": iso.string(from: e.endDate),
            "all_day": e.isAllDay,
        ]
    }
    return ["name": cal.title, "color": hex(cal.cgColor), "events": events]
}

func put(_ cal: EKCalendar, _ body: [String: Any]) {
    let id = cal.calendarIdentifier.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed)!
    var req = URLRequest(url: base.appendingPathComponent("api/sync/calendars/\(id)"))
    req.httpMethod = "PUT"
    req.setValue("application/json", forHTTPHeaderField: "Content-Type")
    req.setValue("Basic " + Data("sync:\(token)".utf8).base64EncodedString(), forHTTPHeaderField: "Authorization")
    req.httpBody = try! JSONSerialization.data(withJSONObject: body)
    let done = DispatchSemaphore(value: 0)
    URLSession.shared.dataTask(with: req) { data, resp, err in
        let code = (resp as? HTTPURLResponse)?.statusCode ?? 0
        let text = data.flatMap { String(data: $0, encoding: .utf8) } ?? (err?.localizedDescription ?? "")
        print("\(cal.title): \(code) \(text)")
        done.signal()
    }.resume()
    done.wait()
}

for cal in store.calendars(for: .event) {
    put(cal, snapshot(cal))
}
