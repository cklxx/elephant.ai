import { createLogger } from "../logger";
import * as debugMode from "../debugMode";

describe("createLogger", () => {
  let warnSpy: ReturnType<typeof vi.spyOn>;
  let errorSpy: ReturnType<typeof vi.spyOn>;
  let logSpy: ReturnType<typeof vi.spyOn>;
  let infoSpy: ReturnType<typeof vi.spyOn>;
  let debugSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    logSpy = vi.spyOn(console, "log").mockImplementation(() => {});
    infoSpy = vi.spyOn(console, "info").mockImplementation(() => {});
    debugSpy = vi.spyOn(debugMode, "isDebugModeEnabled");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("logs error messages with namespace prefix", () => {
    const logger = createLogger("SSE");
    logger.error("Connection failed");
    expect(errorSpy).toHaveBeenCalledWith("[SSE] Connection failed");
  });

  it("logs warn messages with namespace prefix", () => {
    const logger = createLogger("Auth");
    logger.warn("Token expired");
    expect(warnSpy).toHaveBeenCalledWith("[Auth] Token expired");
  });

  it("includes context when provided", () => {
    const logger = createLogger("API");
    const ctx = { status: 404, url: "/test" };
    logger.error("Request failed", ctx);
    expect(errorSpy).toHaveBeenCalledWith("[API] Request failed", ctx);
  });

  it("suppresses debug messages when debug mode is disabled", () => {
    debugSpy.mockReturnValue(false);
    const logger = createLogger("App");
    logger.debug("Trace message");
    expect(logSpy).not.toHaveBeenCalled();
  });

  it("emits debug messages when debug mode is enabled", () => {
    debugSpy.mockReturnValue(true);
    const logger = createLogger("App");
    logger.debug("Trace message");
    expect(logSpy).toHaveBeenCalledWith("[App] Trace message");
  });

  it("suppresses info messages when debug mode is disabled", () => {
    debugSpy.mockReturnValue(false);
    const logger = createLogger("App");
    logger.info("Status update");
    expect(infoSpy).not.toHaveBeenCalled();
  });

  it("emits info messages when debug mode is enabled", () => {
    debugSpy.mockReturnValue(true);
    const logger = createLogger("App");
    logger.info("Status update");
    expect(infoSpy).toHaveBeenCalledWith("[App] Status update");
  });

  it("always emits warn and error regardless of debug mode", () => {
    debugSpy.mockReturnValue(false);
    const logger = createLogger("Core");
    logger.warn("Warning msg");
    logger.error("Error msg");
    expect(warnSpy).toHaveBeenCalledWith("[Core] Warning msg");
    expect(errorSpy).toHaveBeenCalledWith("[Core] Error msg");
  });

  it("creates child loggers with nested namespaces", () => {
    const parent = createLogger("SSE");
    const child = parent.child("Connection");
    child.error("Timeout");
    expect(errorSpy).toHaveBeenCalledWith("[SSE][Connection] Timeout");
  });

  it("supports multi-level nesting", () => {
    const logger = createLogger("App").child("Auth").child("Token");
    logger.warn("Expired");
    expect(warnSpy).toHaveBeenCalledWith("[App][Auth][Token] Expired");
  });

  it("omits context object when empty", () => {
    const logger = createLogger("Test");
    logger.error("No context", {});
    expect(errorSpy).toHaveBeenCalledWith("[Test] No context");
  });
});
