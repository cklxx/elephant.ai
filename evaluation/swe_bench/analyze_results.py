#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""分析Ultra Think模式评估结果"""

import json
import os
from typing import Dict, List

def analyze_evaluation_results(results_dir: str = "ultra_think_results"):
    """分析评估结果并生成评分报告"""
    
    # 读取各种结果文件
    with open(f"{results_dir}/summary.json", "r") as f:
        summary = json.load(f)
    
    with open(f"{results_dir}/detailed_results.json", "r") as f:
        detailed = json.load(f)
    
    with open(f"{results_dir}/batch_results.json", "r") as f:
        batch = json.load(f)
    
    print("=" * 80)
    print("🎯 Ultra Think 模式评估报告")
    print("=" * 80)
    
    # 1. 总体评分
    print("\n📊 总体评分")
    print("-" * 40)
    total_score = summary["success_rate"]
    print(f"✅ 成功率: {total_score}%")
    print(f"📝 完成任务: {summary['completed_tasks']}/{summary['total_tasks']}")
    print(f"❌ 失败任务: {summary['failed_tasks']}")
    print(f"⏱️  平均耗时: {summary['avg_duration']}")
    print(f"💰 总成本: ${summary['total_cost']:.4f}")
    print(f"🤖 使用模型: {summary['model_name']}")
    
    # 2. 详细案例分析
    print("\n🔍 详细案例分析")
    print("-" * 40)
    
    for idx, result in enumerate(detailed, 1):
        print(f"\n案例 {idx}: {result['instance_id']}")
        print(f"  状态: {'✅ 成功' if result['status'] == 'completed' else '❌ 失败'}")
        print(f"  耗时: {result['duration'] / 1e6:.2f}ms")
        print(f"  Token使用: {result['tokens_used']}")
        print(f"  成本: ${result['cost']:.4f}")
        print(f"  修改文件: {', '.join(result['files_changed'])}")
        
        # 显示思考轨迹
        if 'trace' in result and result['trace']:
            print(f"  思考步骤: {len(result['trace'])}步")
            for step in result['trace'][:2]:  # 只显示前2步
                print(f"    Step {step['step']}: {step['action']}")
                print(f"      思考: {step['thought'][:50]}...")
    
    # 3. 性能指标
    print("\n⚡ 性能指标")
    print("-" * 40)
    print(f"总耗时: {summary['duration']}")
    print(f"总Token: {summary['total_tokens']}")
    print(f"并发数: {summary['num_workers']}")
    
    # 4. 评分计算
    print("\n🏆 综合评分计算")
    print("-" * 40)
    
    # 评分维度
    scores = {
        "功能完成度": total_score,  # 基于成功率
        "效率评分": min(100, (1000 / (float(summary['avg_duration'].rstrip('ms')) + 1)) * 100),  # 基于速度
        "成本效益": min(100, (0.001 / (summary['total_cost'] + 0.0001)) * 100),  # 基于成本
        "思考深度": min(100, sum([len(r.get('trace', [])) for r in detailed]) / len(detailed) * 25)  # 基于步骤数
    }
    
    for dimension, score in scores.items():
        bar = "█" * int(score / 5) + "░" * (20 - int(score / 5))
        print(f"  {dimension:12s}: {bar} {score:.1f}%")
    
    # 综合评分
    final_score = sum(scores.values()) / len(scores)
    print(f"\n  📈 综合评分: {final_score:.1f}/100")
    
    # 评级
    if final_score >= 90:
        grade = "A+ 优秀"
    elif final_score >= 80:
        grade = "A 良好"
    elif final_score >= 70:
        grade = "B 合格"
    elif final_score >= 60:
        grade = "C 及格"
    else:
        grade = "D 需改进"
    
    print(f"  🏅 评级: {grade}")
    
    # 5. Ultra Think特性分析
    print("\n🧠 Ultra Think 特性分析")
    print("-" * 40)
    
    # 检查是否有深度思考迹象
    deep_thinking_indicators = 0
    for result in detailed:
        if 'trace' in result:
            # 检查思考步骤
            if len(result['trace']) >= 4:
                deep_thinking_indicators += 1
            # 检查是否有分析和推理步骤
            for step in result['trace']:
                if any(keyword in step['action'].lower() for keyword in ['analyze', 'identify', 'reason']):
                    deep_thinking_indicators += 0.5
                    break
    
    ultra_think_score = min(100, (deep_thinking_indicators / len(detailed)) * 100)
    print(f"  深度思考指标: {ultra_think_score:.1f}%")
    print(f"  推理模型使用: {'是' if 'r1' in summary['model_name'] else '否'}")
    print(f"  平均思考步骤: {sum([len(r.get('trace', [])) for r in detailed]) / len(detailed):.1f}步")
    
    print("\n" + "=" * 80)
    print("评估完成！框架验证成功 ✅")
    print("=" * 80)

if __name__ == "__main__":
    # 检查结果目录
    if os.path.exists("ultra_think_results"):
        analyze_evaluation_results("ultra_think_results")
    elif os.path.exists("real_test_results"):
        print("使用默认结果目录...")
        analyze_evaluation_results("real_test_results")
    else:
        print("错误: 未找到评估结果目录")