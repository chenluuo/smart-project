package com.smartagriculture.control.repository;

import com.smartagriculture.control.domain.CommandStatus;
import com.smartagriculture.control.domain.DeviceCommand;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface DeviceCommandRepository extends JpaRepository<DeviceCommand, Long> {

    Optional<DeviceCommand> findByCommandId(String commandId);

    Optional<DeviceCommand> findByIdempotencyKey(String idempotencyKey);

    List<DeviceCommand> findByDeviceIdAndStatusIn(Long deviceId, List<CommandStatus> statuses);
}
