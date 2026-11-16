using System.Diagnostics;
using System.Text.Json;
using Cage.Windows;

internal static class UpdateWorkingDirectoryTests
{
    public static async Task HoldHelper()
    {
        var release = Environment.GetEnvironmentVariable("CAGE_TEST_UPDATE_RELEASE")
            ?? throw new InvalidOperationException("缺少测试进程释放路径");
        Console.WriteLine(JsonSerializer.Serialize(new {
            pid = Environment.ProcessId,
            workingDirectory = Environment.CurrentDirectory,
            profile = Environment.GetEnvironmentVariable("CLAUDE_CAGE_CONFIG"),
        }));
        await Console.Out.FlushAsync();
        using var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(30));
        while (!File.Exists(release)) await Task.Delay(20, deadline.Token);
    }

    public static async Task LaunchHelper(string helperPath)
    {
        using var child = Process.Start(Backend.StartInfo(helperPath, "--install-update"))
            ?? throw new IOException("无法启动测试更新进程");
        using var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        var ready = await child.StandardOutput.ReadLineAsync(deadline.Token)
            ?? throw new IOException("测试更新进程没有返回状态");
        Console.WriteLine(ready);
    }

    public static async Task Run()
    {
        if (!OperatingSystem.IsWindows()) throw new PlatformNotSupportedException("需要真实 Windows 目录占用语义");
        var root = Path.GetFullPath(Path.Combine(Environment.CurrentDirectory, "_output", "update-cwd-" + Guid.NewGuid().ToString("N")));
        var installed = Path.Combine(root, "installed application");
        var updates = Path.Combine(root, "profile", "updates");
        Directory.CreateDirectory(installed);
        Directory.CreateDirectory(updates);
        foreach (var file in Directory.EnumerateFiles(AppContext.BaseDirectory))
        {
            File.Copy(file, Path.Combine(installed, Path.GetFileName(file)));
            File.Copy(file, Path.Combine(updates, Path.GetFileName(file)));
        }
        var executableName = Path.GetFileName(Environment.ProcessPath!);
        var selectedProfile = Path.Combine(root, "profile", "config.json");
        var release = Path.Combine(root, "release-helper");
        var start = new ProcessStartInfo(Path.Combine(installed, executableName)) {
            UseShellExecute = false, CreateNoWindow = true, WorkingDirectory = installed,
            RedirectStandardOutput = true, RedirectStandardError = true,
        };
        start.ArgumentList.Add("update-helper-probe");
        start.ArgumentList.Add(Path.Combine(updates, executableName));
        start.Environment["CLAUDE_CAGE_CONFIG"] = Path.GetRelativePath(installed, selectedProfile);
        start.Environment["CAGE_TEST_UPDATE_RELEASE"] = release;
        using var controller = Process.Start(start) ?? throw new IOException("无法启动独立测试控制进程");
        using var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        var ready = await controller.StandardOutput.ReadLineAsync(deadline.Token);
        await controller.WaitForExitAsync(deadline.Token);
        if (controller.ExitCode != 0 || ready == null)
            throw new IOException("测试控制进程失败：" + await controller.StandardError.ReadToEndAsync(deadline.Token));
        using var state = JsonDocument.Parse(ready);
        using var helper = Process.GetProcessById(state.RootElement.GetProperty("pid").GetInt32());
        _ = helper.SafeHandle;
        try
        {
            if (state.RootElement.GetProperty("profile").GetString() != selectedProfile)
                throw new IOException("更新进程改变了相对配置路径的含义");
            if (helper.HasExited) throw new IOException("目录移动前更新进程已经退出");
            Console.WriteLine("更新进程实际工作目录：" + state.RootElement.GetProperty("workingDirectory").GetString());
            Directory.Move(installed, installed + ".previous");
            if (!Directory.Exists(installed + ".previous") || helper.HasExited)
                throw new IOException("未能在更新进程运行期间移动安装目录");
            Console.WriteLine("PASS：控制进程退出后，更新进程保持运行，安装目录移动成功；配置路径保持一致。");
        }
        finally
        {
            File.WriteAllText(release, "release owned helper");
            await helper.WaitForExitAsync().WaitAsync(TimeSpan.FromSeconds(10));
        }
        await VerifyPreservationProcess(root);
    }

    private static async Task VerifyPreservationProcess(string root)
    {
        var source = Path.Combine(root, "original files");
        var stage = Path.Combine(root, "restored files");
        foreach (var directory in new[] { source, stage })
        {
            Directory.CreateDirectory(directory);
            File.WriteAllText(Path.Combine(directory, "Cage.install-id"), "io.github.lee-xeo.claude-cage");
            File.WriteAllText(Path.Combine(directory, "program.txt"), "declared program data");
            File.WriteAllText(Path.Combine(directory, "Uninstall.exe"), "never executed; manifest data only");
            var files = new[] { "Cage.install-id", "program.txt" }.Select(path => new {
                path, sha256 = Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(File.ReadAllBytes(Path.Combine(directory, path)))).ToLowerInvariant(),
            });
            File.WriteAllText(Path.Combine(directory, "Cage.install-files.json"), JsonSerializer.Serialize(new { version = 1, files }));
        }
        Directory.CreateDirectory(Path.Combine(source, "CageData"));
        var personal = Path.Combine(source, "CageData", "personal.txt");
        File.WriteAllText(personal, "personal data stays in the original installation");
        var receipt = source + ".cage-installation.json";
        var preserve = typeof(UpdateInstaller).GetMethod("PreserveFiles", System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Static)!;
        await (Task)preserve.Invoke(null, ["prepare", source, stage, receipt, source + ".failed"])!;
        if (!File.Exists(receipt)) throw new IOException("真实文件保留进程未写入恢复记录");
        await (Task)preserve.Invoke(null, ["complete", "", source, receipt, null])!;
        typeof(UpdateInstaller).GetMethod("EnsureInstallationComplete", System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Static)!.Invoke(null, [source]);
        if (File.Exists(receipt) || File.ReadAllText(personal) != "personal data stays in the original installation" || !Directory.Exists(stage))
            throw new IOException("真实恢复进程改变了原有个人数据或遗留恢复记录");
        Directory.Move(source, source + ".after-complete");
        Console.WriteLine("PASS：生产 PreserveFiles 启动真实 Windows PowerShell 完成 prepare/complete，个人数据保留，子进程退出后目录可移动。");
    }
}
