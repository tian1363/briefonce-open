import AppKit
import Foundation
import Vision

struct Result: Encodable {
    let index: Int
    let text: String
    let confidence: Float
}

var results: [Result] = []
for (index, path) in CommandLine.arguments.dropFirst().enumerated() {
    guard let image = NSImage(contentsOfFile: path),
          let cgImage = image.cgImage(forProposedRect: nil, context: nil, hints: nil) else {
        continue
    }
    let request = VNRecognizeTextRequest()
    request.recognitionLevel = .accurate
    request.recognitionLanguages = ["zh-Hans", "en-US"]
    request.usesLanguageCorrection = true
    do {
        try VNImageRequestHandler(cgImage: cgImage).perform([request])
        let candidates = (request.results ?? []).compactMap { $0.topCandidates(1).first }
        let text = candidates.map { $0.string }.joined(separator: " ")
        let confidence = candidates.map { $0.confidence }.min() ?? 0
        results.append(Result(index: index, text: text, confidence: confidence))
    } catch {
        continue
    }
}

let encoder = JSONEncoder()
if let data = try? encoder.encode(results), let output = String(data: data, encoding: .utf8) {
    print(output)
}
