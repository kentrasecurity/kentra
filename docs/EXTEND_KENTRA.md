# Extending Kentra

### Adding New CRD Types

For more info on this topic, refer to this [example](./NEW_CRD_EXAMPLE.md).

### Adding New Tool Support

To support a new security tool:

1. Create Tool Image: Build or use existing Docker image
2. Define ToolSpec: Add entry to `kentra-tool-specs` ConfigMap
3. Test: Create an attack with new tool
4. Document: Update tool configuration examples

Warning: sometimes tools require its specific parsing method. Check out for example the port and endpoint separator for [nmap](). 