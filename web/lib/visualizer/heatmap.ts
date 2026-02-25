/**
 * Heatmap manager tracks activity on files/folders
 * and computes heat scores for visualization
 */
export class HeatmapManager {
  private activityMap = new Map<string, number[]>();
  private readonly WINDOW_SIZE = 60000; // 1 minute activity window

  /**
   * Record an activity on a path
   */
  recordActivity(path: string): void {
    const now = Date.now();
    const activities = this.activityMap.get(path) || [];

    // Add new activity timestamp
    activities.push(now);

    // Remove activities outside the window
    const filtered = activities.filter((timestamp) => now - timestamp < this.WINDOW_SIZE);

    this.activityMap.set(path, filtered);
  }

  /**
   * Get heat score for a path (0 = cold, 1 = hot)
   */
  getHeatScore(path: string): number {
    const activities = this.activityMap.get(path) || [];

    // Normalize to 0-1 range (assume 10 activities = max heat)
    return Math.min(activities.length / 10, 1);
  }

  /**
   * Get color for a given heat score
   */
  getHeatColor(score: number): string {
    if (score < 0.2) {
      return '#5588aa'; // Cold blue
    } else if (score < 0.5) {
      return '#88aa77'; // Warm green
    } else if (score < 0.8) {
      return '#ddaa44'; // Hot yellow
    } else {
      return '#ff5533'; // Very hot red
    }
  }

  /**
   * Clear all activity data
   */
  clear(): void {
    this.activityMap.clear();
  }

  /**
   * Get all paths with activity
   */
  getActivePaths(): string[] {
    return Array.from(this.activityMap.keys());
  }

  /**
   * Get total activity count across all paths
   */
  getTotalActivityCount(): number {
    let total = 0;
    for (const activities of this.activityMap.values()) {
      total += activities.length;
    }
    return total;
  }
}
